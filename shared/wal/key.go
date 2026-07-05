package wal

import (
	"encoding/binary"
	"fmt"
)

func (w *WalEntry) DedupKey() ([16]byte, error) {
	var key [16]byte

	key[0] = w.Type
	key[1] = w.From
	switch w.Type {
	case WalItem:
		key[2] = w.Item.Game
		binary.LittleEndian.PutUint32(key[3:7], w.Item.Key)
		binary.LittleEndian.PutUint32(key[7:11], w.Item.Nonce)
	case WalEvent:
		binary.LittleEndian.PutUint32(key[2:6], w.Event.ID)
	default:
		return key, fmt.Errorf("unknown WAL entry type: %d", w.Type)
	}

	return key, nil
}
