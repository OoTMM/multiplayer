package shared

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
)

type PlayerName struct {
	ID   [16]byte
	Name [8]byte
}

func SerializePlayerNames(names []PlayerName) []byte {
	buf := make([]byte, 0)

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(names)))
	for _, m := range names {
		buf = append(buf, m.ID[:]...)
		buf = append(buf, m.Name[:]...)
	}
	return buf
}

func DeserializePlayerNames(data []byte) ([]PlayerName, error) {
	r := NewBytesReader(data)
	count := r.ReadUint32()
	meta := make([]PlayerName, count)
	for i := uint32(0); i < count; i++ {
		r.Read(meta[i].ID[:])
		r.Read(meta[i].Name[:])
	}
	return meta, r.Err()
}

type PlayerNamesStore struct {
	path string
	data map[[16]byte]*PlayerName
	mu   sync.RWMutex
}

func (store *PlayerNamesStore) load() error {
	data, err := os.ReadFile(store.path)
	if err != nil {
		return nil
	}

	names, err := DeserializePlayerNames(data)
	if err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	for i := range names {
		store.data[names[i].ID] = &names[i]
	}

	return nil
}

func OpenPlayerNamesStore(path string) (*PlayerNamesStore, error) {
	store := &PlayerNamesStore{
		path: path,
		data: make(map[[16]byte]*PlayerName),
	}

	err := store.load()
	if err != nil {
		fmt.Printf("warn: invalid names file")
	}

	return store, nil
}

func (store *PlayerNamesStore) serializeNoLock() []byte {
	names := make([]PlayerName, 0, len(store.data))
	for _, entry := range store.data {
		names = append(names, *entry)
	}
	return SerializePlayerNames(names)
}

func (store *PlayerNamesStore) Serialize() []byte {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.serializeNoLock()
}

func (store *PlayerNamesStore) Add(entry *PlayerName) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	/* Check for existing entry */
	existing, exists := store.data[entry.ID]
	if exists && existing.Name == entry.Name {
		return false
	}

	/* Update */
	store.data[entry.ID] = entry

	/* Persist to disk */
	serialized := store.serializeNoLock()
	err := os.WriteFile(store.path, serialized, 0644)
	if err != nil {
		fmt.Printf("warn: failed to write names file: %v\n", err)
	}

	return true
}

func (store *PlayerNamesStore) Get(id [16]byte) *PlayerName {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return store.data[id]
}
