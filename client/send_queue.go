package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

type SendQueue struct {
	file    *os.File
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	entries map[[16]byte][]byte
}

func loadSendQueue(sq *SendQueue) error {
	var id [16]byte
	var lengthBuf [4]byte

	for {
		/* Read ID prefix */
		_, err := io.ReadFull(sq.file, id[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("send queue: failed to read entry ID: %v", err)
		}

		/* Read length */
		_, err = io.ReadFull(sq.file, lengthBuf[:])
		if err != nil {
			return fmt.Errorf("send queue: failed to read entry length: %v", err)
		}
		length := binary.LittleEndian.Uint32(lengthBuf[:])

		/* Read data */
		data := make([]byte, length)
		_, err = io.ReadFull(sq.file, data)
		if err != nil {
			return fmt.Errorf("send queue: failed to read entry data: %v", err)
		}

		/* Store the entry */
		sq.entries[id] = data
	}

	return nil
}

func OpenSendQueue(path string) (*SendQueue, error) {
	/* Open the send queue file */
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("send queue: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sq := &SendQueue{
		file:    file,
		ctx:     ctx,
		cancel:  cancel,
		entries: make(map[[16]byte][]byte),
	}

	err = loadSendQueue(sq)
	if err != nil {
		sq.Close()
		return nil, err
	}

	return sq, nil
}

func (sq *SendQueue) Ack(id [16]byte) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if len(sq.entries) == 0 {
		return
	}

	/* Remove the entry */
	delete(sq.entries, id)

	/* If there are no more entries, truncate */
	if len(sq.entries) == 0 {
		err := sq.file.Truncate(0)
		if err != nil {
			fmt.Printf("Failed to truncate send queue: %v\n", err)
		} else {
			sq.file.Seek(0, io.SeekStart)
		}
	}

	/* Sync the file */
	err := sq.file.Sync()
	if err != nil {
		fmt.Printf("Failed to sync send queue: %v\n", err)
	}
}

func (sq *SendQueue) Add(id [16]byte, data []byte) error {
	var lengthBuf [4]byte

	sq.mu.Lock()
	defer sq.mu.Unlock()

	/* Check for duplicate ID */
	if _, exists := sq.entries[id]; exists {
		return nil
	}

	/* Write the entry to the end of the file */
	_, err := sq.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of send queue: %v", err)
	}
	_, err = sq.file.Write(id[:])
	if err != nil {
		return fmt.Errorf("failed to write entry ID to send queue: %v", err)
	}
	binary.LittleEndian.PutUint32(lengthBuf[:], uint32(len(data)))
	_, err = sq.file.Write(lengthBuf[:])
	if err != nil {
		return fmt.Errorf("failed to write entry length to send queue: %v", err)
	}
	_, err = sq.file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write entry data to send queue: %v", err)
	}

	/* Sync the file */
	err = sq.file.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync send queue: %v", err)
	}

	/* Store in the in-memory map */
	sq.entries[id] = data

	return nil
}

func (sq *SendQueue) Close() {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	sq.cancel()
	sq.file.Close()
}

func (sq *SendQueue) Pending() [][]byte {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	pending := make([][]byte, 0, len(sq.entries))
	for _, data := range sq.entries {
		pending = append(pending, data)
	}
	return pending
}
