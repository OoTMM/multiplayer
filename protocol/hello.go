package protocol

import (
	"encoding/binary"
	"fmt"
)

const NET_PROTO = 0x00000001
const NET_MAGIC = "OoTMM2\x00\xfe"

type ClientHello struct {
	SessionID      [16]byte
	SessionSecret  uint32
	WalIndex       uint32
	PlayerName     [8]byte
	PlayerUniqueID uint64
	PlayerID       uint8
}

func SerializeClientHello(hello *ClientHello) []byte {
	buf := make([]byte, 0, 64)
	buf = append(buf, NET_MAGIC[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, NET_PROTO)
	buf = append(buf, hello.SessionID[:]...)
	buf = binary.LittleEndian.AppendUint32(buf, hello.SessionSecret)
	buf = binary.LittleEndian.AppendUint32(buf, hello.WalIndex)
	buf = append(buf, hello.PlayerName[:]...)
	buf = binary.LittleEndian.AppendUint64(buf, hello.PlayerUniqueID)
	buf = append(buf, hello.PlayerID)
	return buf
}

func DeserializeClientHello(data []byte) (*ClientHello, error) {
	if len(data) < 8+4+16+4+4+8+8+1 {
		return nil, fmt.Errorf("data too short for Hello")
	}
	magic := string(data[0:8])
	if magic != NET_MAGIC {
		return nil, fmt.Errorf("invalid magic")
	}
	proto := binary.LittleEndian.Uint32(data[8:12])
	if proto != NET_PROTO {
		return nil, fmt.Errorf("invalid protocol version")
	}
	var hello ClientHello
	copy(hello.SessionID[:], data[12:28])
	hello.SessionSecret = binary.LittleEndian.Uint32(data[28:32])
	hello.WalIndex = binary.LittleEndian.Uint32(data[32:36])
	copy(hello.PlayerName[:], data[36:44])
	hello.PlayerUniqueID = binary.LittleEndian.Uint64(data[44:52])
	hello.PlayerID = data[52]
	return &hello, nil
}
