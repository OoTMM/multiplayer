package game

type SessionInfo struct {
	SessionID     [16]byte
	SessionSecret [8]byte
	PlayerID      [16]byte
	PlayerName    [8]byte
	WorldID       uint8
	Multiplayer   bool
}

func ReadSessionInfo(data []byte) *SessionInfo {
	var info SessionInfo

	if len(data) < 50 {
		return nil
	}

	copy(info.SessionID[:], data[0:16])
	copy(info.SessionSecret[:], data[16:24])
	copy(info.PlayerID[:], data[24:40])
	copy(info.PlayerName[:], data[40:48])
	info.WorldID = data[48]
	info.Multiplayer = (data[49] != 0)

	return &info
}
