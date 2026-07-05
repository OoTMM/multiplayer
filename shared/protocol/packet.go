package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/OoTMM/multiplayer/shared/wal"
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

type ServerHello struct {
	Magic   [8]byte
	Version uint32
}

type ServerWal struct {
	Index uint32
	Entry *wal.WalEntry
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

func ParseServerHello(data []byte) (*ServerHello, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("server hello too short")
	}
	var hello ServerHello
	copy(hello.Magic[:], data[0:8])
	hello.Version = binary.LittleEndian.Uint32(data[8:12])
	return &hello, nil
}

func (hello *ServerHello) Serialize() []byte {
	data := make([]byte, 12)
	copy(data[0:8], hello.Magic[:])
	binary.LittleEndian.PutUint32(data[8:12], hello.Version)
	return data
}

func ParseServerWal(data []byte) (*ServerWal, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("server wal too short")
	}
	var x ServerWal
	x.Index = binary.LittleEndian.Uint32(data[0:4])
	entry, err := wal.Parse(data[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize wal entry: %v", err)
	}
	x.Entry = entry
	return &x, nil
}

func (x *ServerWal) Serialize() ([]byte, error) {
	entryData, err := x.Entry.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize wal entry: %v", err)
	}
	data := make([]byte, 4+len(entryData))
	binary.LittleEndian.PutUint32(data[0:4], x.Index)
	copy(data[4:], entryData)
	return data, nil
}
