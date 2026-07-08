package wal

import (
	"encoding/binary"
	"fmt"
)

func Parse(data []byte) (*WalEntry, error) {
	if len(data) < 26 {
		return nil, fmt.Errorf("WAL entry too short")
	}

	var entry WalEntry
	entry.Type = data[0]
	copy(entry.PlayerID[:], data[1:17])
	copy(entry.PlayerName[:], data[17:25])
	entry.From = data[25]

	switch entry.Type {
	case WalItem:
		if len(data) < 40 {
			return nil, fmt.Errorf("WAL item entry too short")
		}
		entry.Item.To = data[26]
		entry.Item.Game = data[27]
		entry.Item.GI = binary.LittleEndian.Uint16(data[28:30])
		entry.Item.Flags = binary.LittleEndian.Uint16(data[30:32])
		entry.Item.Key = binary.LittleEndian.Uint32(data[32:36])
		entry.Item.Nonce = binary.LittleEndian.Uint32(data[36:40])
	case WalEvent:
		if len(data) < 30 {
			return nil, fmt.Errorf("WAL event entry too short")
		}
		entry.Event.ID = binary.LittleEndian.Uint32(data[26:30])
	default:
		return nil, fmt.Errorf("unknown WAL entry type: %d", entry.Type)
	}

	return &entry, nil
}

func (e *WalEntry) Serialize() ([]byte, error) {
	buf := make([]byte, 0, 16)

	buf = append(buf, e.Type)
	buf = append(buf, e.PlayerID[:]...)
	buf = append(buf, e.PlayerName[:]...)
	buf = append(buf, e.From)

	switch e.Type {
	case WalItem:
		buf = append(buf, e.Item.To)
		buf = append(buf, e.Item.Game)
		buf = binary.LittleEndian.AppendUint16(buf, e.Item.GI)
		buf = binary.LittleEndian.AppendUint16(buf, e.Item.Flags)
		buf = binary.LittleEndian.AppendUint32(buf, e.Item.Key)
		buf = binary.LittleEndian.AppendUint32(buf, e.Item.Nonce)
	case WalEvent:
		buf = binary.LittleEndian.AppendUint32(buf, e.Event.ID)
	default:
		return nil, fmt.Errorf("unknown WAL entry type: %d", e.Type)
	}

	return buf, nil
}
