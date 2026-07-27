package wal

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
	notify  chan struct{}
}

type WALStream struct {
	ctx   context.Context
	wal   *WAL
	index uint32
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

		length = binary.LittleEndian.Uint32(lengthBuf[:])
		data := make([]byte, length)

		_, err = io.ReadFull(wal.file, data)
		if err != nil {
			return fmt.Errorf("wal: failed to read entry data: %v", err)
		}

		entry, err := Parse(data)
		if err != nil {
			return fmt.Errorf("wal: failed to deserialize entry")
		}

		dedupKey, err := entry.DedupKey()
		if err != nil {
			return fmt.Errorf("wal: failed to compute deduplication key: %v", err)
		}

		wal.set[dedupKey] = struct{}{}
		wal.entries = append(wal.entries, entry)
		wal.count++
	}
	return nil
}

func OpenWAL(ctx context.Context, path string) (*WAL, error) {
	ctx, cancel := context.WithCancel(ctx)

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
		notify:  make(chan struct{}),
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
	data, err := entry.Serialize()
	if err != nil {
		return fmt.Errorf("wal: failed to marshal entry: %v", err)
	}

	dedupKey, err := entry.DedupKey()
	if err != nil {
		return fmt.Errorf("wal: failed to compute deduplication key: %v", err)
	}

	/* Prepare length-prefixed format: [4 bytes length][data] */
	lengthBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lengthBuf, uint32(len(data)))

	/* Now we actually need to lock */
	wal.mu.Lock()
	defer wal.mu.Unlock()

	/* Check for a matching ID */
	if _, exists := wal.set[dedupKey]; exists {
		return nil
	}

	/* Write to the WAL file */
	pos, err := wal.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("wal: failed to seek in file: %v", err)
	}

	/* Write length prefix */
	_, err = wal.file.Write(lengthBuf)
	if err != nil {
		wal.file.Truncate(pos)
		wal.file.Seek(pos, io.SeekStart)
		return fmt.Errorf("wal: failed to write entry length to file: %v", err)
	}

	/* Write entry data */
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
	wal.set[dedupKey] = struct{}{}
	wal.entries = append(wal.entries, entry)
	wal.count++
	close(wal.notify)
	wal.notify = make(chan struct{})

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

func (wal *WAL) Subscribe(ctx context.Context, index uint32) *WALStream {
	stream := &WALStream{
		ctx:   ctx,
		wal:   wal,
		index: index,
	}

	return stream
}

func (stream *WALStream) Next() (*WalEntry, uint32, error) {
	for {
		stream.wal.mu.RLock()
		ch := stream.wal.notify
		stream.wal.mu.RUnlock()

		e := stream.wal.Get(stream.index)
		if e != nil {
			entry := e
			index := stream.index
			stream.index++
			return entry, index, nil
		}

		select {
		case <-ch:
		case <-stream.wal.ctx.Done():
			return nil, 0, fmt.Errorf("wal stream closed")
		case <-stream.ctx.Done():
			return nil, 0, fmt.Errorf("wal stream canceled")
		}
	}
}
