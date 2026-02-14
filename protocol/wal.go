package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type WAL struct {
	file    *os.File
	mu      sync.RWMutex
	set     map[[16]byte]struct{}
	entries []*WalEntry
	count   uint32
}

func (wal *WAL) load() error {
	decoder := json.NewDecoder(wal.file)
	for {
		var entry WalEntry
		err := decoder.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("wal: failed to decode entry: %v", err)
		}
		wal.set[entry.ID] = struct{}{}
		wal.entries = append(wal.entries, &entry)
		wal.count++
	}
	return nil
}

func OpenWAL(path string) (*WAL, error) {
	/* Open the WAL file */
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("wal: %v", err)
	}

	wal := &WAL{
		file:    file,
		set:     make(map[[16]byte]struct{}),
		entries: make([]*WalEntry, 0),
		count:   0,
	}

	/* Load existing entries */
	err = wal.load()
	if err != nil {
		wal.Close()
		return nil, err
	}

	return wal, nil
}

func (wal *WAL) ingest(entry *WalEntry) error {
	/* Marshall - this doesn't need to be locked */
	data, err := json.Marshal(entry)
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

	return nil
}

func (wal *WAL) Append(entry *WalEntry) error {
	err := wal.ingest(entry)
	if err != nil {
		return err
	}

	// TODO: Notify subscribers, etc

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
	wal.mu.Lock()
	defer wal.mu.Unlock()
	return wal.file.Close()
}
