package daemon

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
)

type MsgType string

const (
	MsgTypeGameStart MsgType = "GAME_START"
	MsgTypeGameEnd   MsgType = "GAME_END"
	MsgTypeInfoItem  MsgType = "INFO_ITEM"
)

/* Fat message structure */
type Msg struct {
	Type       MsgType `json:"type"`
	SessionID  string  `json:"sessionId,omitempty"`
	PlayerID   string  `json:"playerId,omitempty"`
	PlayerName string  `json:"playerName,omitempty"`
	WorldID    int     `json:"worldId,omitempty"`
	Item       string  `json:"item,omitempty"`
	Location   string  `json:"location,omitempty"`
}

func sendMsg(conn net.Conn, msg *Msg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var header [4]byte
	size := uint32(len(data))
	binary.LittleEndian.PutUint32(header[:], size)
	_, err = conn.Write(header[:])
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func recvMsg(conn net.Conn) (*Msg, error) {
	var header [4]byte
	_, err := io.ReadFull(conn, header[:])
	if err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[:])
	data := make([]byte, size)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return nil, err
	}
	var msg Msg
	err = json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
