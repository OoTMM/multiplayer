package wal

const (
	WalItem  uint8 = 0x01
	WalEvent uint8 = 0x02
)

type WalEntry struct {
	Type       uint8
	PlayerID   [16]byte
	PlayerName [8]byte
	From       uint8

	Item  WalEntryItem
	Event WalEntryEvent
}

type WalEntryItem struct {
	To    uint8
	Game  uint8
	GI    uint16
	Flags uint16
	Key   uint32
	Nonce uint32
}

type WalEntryEvent struct {
	ID uint32
}
