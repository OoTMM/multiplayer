package protocol

type WalEntry struct {
	ID   string   `json:"id"`
	Type string   `json:"type"`
	Item *WalItem `json:"item,omitempty"`
}

type WalItem struct {
	PlayerID string `json:"playerId"`
	Key      uint32 `json:"key"`
	ItemID   uint16 `json:"itemId"`
	From     uint8  `json:"from"`
	To       uint8  `json:"to"`
	GameID   uint8  `json:"gameId"`
}
