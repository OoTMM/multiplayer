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

type ClientPosition struct {
	Key uint16
	X   int16
	Y   int16
	Z   int16
}

type ServerPosition struct {
	ID   [16]byte
	Name [8]byte
	Key  uint16
	X    int16
	Y    int16
	Z    int16
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

func ParseClientPosition(data []byte) (*ClientPosition, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("client position too short")
	}
	var pos ClientPosition
	pos.Key = binary.LittleEndian.Uint16(data[0:2])
	pos.X = int16(binary.LittleEndian.Uint16(data[2:4]))
	pos.Y = int16(binary.LittleEndian.Uint16(data[4:6]))
	pos.Z = int16(binary.LittleEndian.Uint16(data[6:8]))
	return &pos, nil
}

func (pos *ClientPosition) Serialize() []byte {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint16(data[0:2], pos.Key)
	binary.LittleEndian.PutUint16(data[2:4], uint16(pos.X))
	binary.LittleEndian.PutUint16(data[4:6], uint16(pos.Y))
	binary.LittleEndian.PutUint16(data[6:8], uint16(pos.Z))
	return data
}

func ParseServerPosition(data []byte) (*ServerPosition, error) {
	if len(data) < 34 {
		return nil, fmt.Errorf("server position too short")
	}
	var pos ServerPosition
	copy(pos.ID[:], data[0:16])
	copy(pos.Name[:], data[16:24])
	pos.Key = binary.LittleEndian.Uint16(data[24:26])
	pos.X = int16(binary.LittleEndian.Uint16(data[26:28]))
	pos.Y = int16(binary.LittleEndian.Uint16(data[28:30]))
	pos.Z = int16(binary.LittleEndian.Uint16(data[30:32]))
	return &pos, nil
}

func (pos *ServerPosition) Serialize() []byte {
	data := make([]byte, 34)
	copy(data[0:16], pos.ID[:])
	copy(data[16:24], pos.Name[:])
	binary.LittleEndian.PutUint16(data[24:26], pos.Key)
	binary.LittleEndian.PutUint16(data[26:28], uint16(pos.X))
	binary.LittleEndian.PutUint16(data[28:30], uint16(pos.Y))
	binary.LittleEndian.PutUint16(data[30:32], uint16(pos.Z))
	return data
}
