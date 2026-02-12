package protocol

import "encoding/binary"

const WAL_ENTRY_ITEM = 0x01

type WalEntry struct {
	UUID [16]byte
	Type uint8
	Item *WalItem
}

type WalItem struct {
	PlayerFromUniqueID uint64
	Key                uint32
	ItemID             uint16
	PlayerFrom         uint8
	PlayerTo           uint8
	GameID             uint8
}

func serializeItem(item *WalItem) []byte {
	data := make([]byte, 17)
	binary.BigEndian.PutUint64(data[0:8], item.PlayerFromUniqueID)
	binary.BigEndian.PutUint32(data[8:12], item.Key)
	binary.BigEndian.PutUint16(data[12:14], item.ItemID)
	data[14] = item.PlayerFrom
	data[15] = item.PlayerTo
	data[16] = item.GameID
	return data
}

func serializeSubtype(entry *WalEntry) []byte {
	switch entry.Type {
	case WAL_ENTRY_ITEM:
		return serializeItem(entry.Item)
	default:
		return nil
	}
}

func Serialize(entry *WalEntry) []byte {
	header := make([]byte, 17, 64)
	copy(header[0:16], entry.UUID[:])
	header[16] = entry.Type
	data := serializeSubtype(entry)
	return append(header, data...)
}
