package shared

import (
	"encoding/binary"
	"fmt"
)

type ClientHello struct {
	SessionID     [16]byte
	SessionSecret uint32
	PlayerID      [16]byte
	PlayerName    [8]byte
	WalIndex      uint32
	WorldID       uint8
}

func SerializeClientHello(hello *ClientHello) []byte {
	buf := make([]byte, 0, 64)

	buf = append(buf, NetOpHello)
	buf = append(buf, Magic...)
	buf = binary.LittleEndian.AppendUint32(buf, Version)
	buf = append(buf, hello.SessionID[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, hello.SessionSecret)
	buf = append(buf, hello.PlayerID[:]...)
	buf = append(buf, hello.PlayerName[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, hello.WalIndex)
	buf = append(buf, hello.WorldID)

	return buf
}

func DeserializeClientHello(data []byte) (*ClientHello, error) {
	reader := NewBytesReader(data)
	op := reader.ReadUint8()
	if op != NetOpHello {
		return nil, fmt.Errorf("invalid operation code: expected %02x, got %02x", NetOpHello, op)
	}
	magic := reader.ReadBytes(len(Magic))
	if string(magic) != Magic {
		return nil, fmt.Errorf("invalid magic: expected %q, got %q", Magic, string(magic))
	}
	version := reader.ReadUint32()
	if version != Version {
		return nil, fmt.Errorf("unsupported protocol version: expected %d, got %d", Version, version)
	}
	hello := &ClientHello{}
	reader.Read(hello.SessionID[:])
	hello.SessionSecret = reader.ReadUint32()
	reader.Read(hello.PlayerID[:])
	reader.Read(hello.PlayerName[:])
	hello.WalIndex = reader.ReadUint32()
	hello.WorldID = reader.ReadUint8()

	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("failed to deserialize ClientHello: %v", err)
	}

	return hello, nil
}
