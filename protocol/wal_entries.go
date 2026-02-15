package protocol

import (
	"encoding/binary"
	"fmt"
)

const (
	WalTypeItem = 0x01
)

type WalEntry struct {
	ID   [16]byte
	Type uint8
	Item *WalItem
}

type WalItem struct {
	PlayerID [16]byte
	From     uint8
	To       uint8
	Key      uint32
	ItemID   uint16
	GameID   uint8
}

func serializeWalItem(item *WalItem) []byte {
	buf := make([]byte, 25)
	copy(buf[0:16], item.PlayerID[:])
	buf[16] = item.From
	buf[17] = item.To
	binary.LittleEndian.PutUint32(buf[18:22], item.Key)
	binary.LittleEndian.PutUint16(buf[22:24], item.ItemID)
	buf[24] = item.GameID
	return buf
}

func SerializeWalEntry(entry *WalEntry) ([]byte, error) {
	var data []byte
	buf := make([]byte, 17)
	copy(buf[0:16], entry.ID[:])
	buf[16] = entry.Type

	switch entry.Type {
	case WalTypeItem:
		data = serializeWalItem(entry.Item)
	}

	return append(buf, data...), nil
}

func deserializeWalItem(data []byte, entry *WalEntry) error {
	if len(data) < 25 {
		return fmt.Errorf("invalid WAL item data length: %d bytes", len(data))
	}
	item := &WalItem{}
	copy(item.PlayerID[:], data[0:16])
	item.From = data[16]
	item.To = data[17]
	item.Key = binary.LittleEndian.Uint32(data[18:22])
	item.ItemID = binary.LittleEndian.Uint16(data[22:24])
	item.GameID = data[24]
	entry.Item = item
	return nil
}

func DeserializeWalEntry(data []byte) (*WalEntry, error) {
	if len(data) < 17 {
		return nil, fmt.Errorf("invalid WAL entry data length: %d bytes", len(data))
	}
	entry := &WalEntry{
		Type: data[16],
	}
	copy(entry.ID[:], data[0:16])
	nextData := data[17:]

	var err error
	switch entry.Type {
	case WalTypeItem:
		err = deserializeWalItem(nextData, entry)
	default:
		return nil, fmt.Errorf("unknown WAL entry type: %d", entry.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize WAL entry: %v", err)
	}

	return entry, nil
}
