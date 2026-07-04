package protocol

import (
	"encoding/binary"
	"fmt"
)

type Packet struct {
	Op   Opcode
	Data []byte
}

type ClientHello struct {
	Magic         [8]byte
	Version       uint32
	SessionID     [16]byte
	SessionSecret [8]byte
	PlayerID      [16]byte
	PlayerName    [8]byte
	WorldID       uint8
	WalIndex      uint32
}

func ParseClientHello(data []byte) (*ClientHello, error) {
	if len(data) < 65 {
		return nil, fmt.Errorf("client hello too short")
	}
	var hello ClientHello
	copy(hello.Magic[:], data[0:8])
	hello.Version = binary.LittleEndian.Uint32(data[8:12])
	copy(hello.SessionID[:], data[12:28])
	copy(hello.SessionSecret[:], data[28:36])
	copy(hello.PlayerID[:], data[36:52])
	copy(hello.PlayerName[:], data[52:60])
	hello.WorldID = data[60]
	hello.WalIndex = binary.LittleEndian.Uint32(data[61:65])
	return &hello, nil
}

func (hello *ClientHello) Serialize() []byte {
	data := make([]byte, 65)
	copy(data[0:8], hello.Magic[:])
	binary.LittleEndian.PutUint32(data[8:12], hello.Version)
	copy(data[12:28], hello.SessionID[:])
	copy(data[28:36], hello.SessionSecret[:])
	copy(data[36:52], hello.PlayerID[:])
	copy(data[52:60], hello.PlayerName[:])
	data[60] = hello.WorldID
	binary.LittleEndian.PutUint32(data[61:65], hello.WalIndex)
	return data
}
