package protocol

import "encoding/binary"

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
