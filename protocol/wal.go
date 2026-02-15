package protocol

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

type WAL struct {
	ctx     context.Context
	cancel  context.CancelFunc
	file    *os.File
	mu      sync.RWMutex
	set     map[[16]byte]struct{}
	entries []*WalEntry
	count   uint32
	streams map[*WALStream]struct{}
}

type WALStream struct {
	notify   chan struct{}
	index    uint32
	cancel   context.CancelFunc
	callback func(uint32, *WalEntry)
}

func (wal *WAL) load() error {
	var length uint32
	var lengthBuf [4]byte

	for {
		_, err := io.ReadFull(wal.file, lengthBuf[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("wal: failed to read entry length: %v", err)
		}

		data := make([]byte, length)
		length = binary.LittleEndian.Uint32(lengthBuf[:])
		_, err = io.ReadFull(wal.file, data)
		if err != nil {
			return fmt.Errorf("wal: failed to read entry data: %v", err)
		}

		entry, err := DeserializeWalEntry(data)
		if entry == nil {
			return fmt.Errorf("wal: failed to deserialize entry")
		}

		wal.set[entry.ID] = struct{}{}
		wal.entries = append(wal.entries, entry)
		wal.count++
	}
	return nil
}

func OpenWAL(path string) (*WAL, error) {
	ctx, cancel := context.WithCancel(context.Background())

	/* Open the WAL file */
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("wal: %v", err)
	}

	wal := &WAL{
		ctx:     ctx,
		cancel:  cancel,
		file:    file,
		set:     make(map[[16]byte]struct{}),
		entries: make([]*WalEntry, 0),
		count:   0,
		streams: make(map[*WALStream]struct{}),
	}

	/* Load existing entries */
	err = wal.load()
	if err != nil {
		wal.Close()
		return nil, err
	}

	return wal, nil
}

func (wal *WAL) Append(entry *WalEntry) error {
	data, err := SerializeWalEntry(entry)
	if err != nil {
		return fmt.Errorf("wal: failed to marshal entry: %v", err)
	}
	data = append(data, '\n')

	/* Now we actually need to lock */
	wal.mu.Lock()
	defer wal.mu.Unlock()

	/* Check for a matching ID */
	if _, exists := wal.set[entry.ID]; exists {
		return nil
	}

	/* Write to the WAL file */
	pos, err := wal.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("wal: failed to seek in file: %v", err)
	}

	_, err = wal.file.Write(data)
	if err != nil {
		wal.file.Truncate(pos)
		wal.file.Seek(pos, io.SeekStart)
		return fmt.Errorf("wal: failed to write entry to file: %v", err)
	}

	/* Sync the file to ensure durability */
	err = wal.file.Sync()
	if err != nil {
		wal.file.Truncate(pos)
		wal.file.Seek(pos, io.SeekStart)
		return fmt.Errorf("wal: failed to sync file: %v", err)
	}

	/* Add to the in-memory set and list */
	wal.set[entry.ID] = struct{}{}
	wal.entries = append(wal.entries, entry)
	wal.count++

	/* Notify streams */
	for stream, _ := range wal.streams {
		select {
		case stream.notify <- struct{}{}:
		default:
		}
	}

	return nil
}

func (wal *WAL) Get(index uint32) *WalEntry {
	wal.mu.RLock()
	defer wal.mu.RUnlock()
	if index >= wal.count {
		return nil
	}
	return wal.entries[index]
}

func (wal *WAL) Count() uint32 {
	wal.mu.RLock()
	defer wal.mu.RUnlock()
	return wal.count
}

func (wal *WAL) Close() error {
	wal.cancel()
	wal.mu.Lock()
	defer wal.mu.Unlock()
	return wal.file.Close()
}

func (wal *WAL) Subscribe(index uint32, callback func(uint32, *WalEntry)) *WALStream {
	ctx, cancel := context.WithCancel(wal.ctx)
	stream := &WALStream{
		notify:   make(chan struct{}, 1),
		index:    index,
		cancel:   cancel,
		callback: callback,
	}

	/* Register the stream */
	wal.mu.Lock()
	wal.streams[stream] = struct{}{}
	wal.mu.Unlock()

	/* Start the stream loop */
	go func() {
		defer stream.cancel()

		defer func() {
			wal.mu.Lock()
			delete(wal.streams, stream)
			wal.mu.Unlock()
		}()

		for {
			for {
				/* Check for cancellation */
				select {
				case <-ctx.Done():
					return
				default:
				}

				/* Get the latest entry */
				entry := wal.Get(stream.index)
				if entry == nil {
					break
				}
				stream.callback(stream.index, entry)
				stream.index++
			}

			/* Wait for a notification or cancellation */
			select {
			case <-ctx.Done():
				return
			case <-stream.notify:
			}
		}
	}()

	return stream
}

func (stream *WALStream) Close() {
	stream.cancel()
}
