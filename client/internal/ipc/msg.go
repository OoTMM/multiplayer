package ipc

import (
	"encoding/binary"
	"fmt"
)

type Opcode uint8

const (
	OpHello    Opcode = 0x01
	OpWal      Opcode = 0x02
	OpWalQuery Opcode = 0x03
	OpWalAck   Opcode = 0x04
)

type Message struct {
	Seq     uint32
	Op      Opcode
	Payload []byte
}

type MessageBodyHelloIn struct {
	Magic         [8]byte
	SessionID     [16]byte
	SessionSecret [8]byte
	PlayerID      [16]byte
	PlayerName    [8]byte
	WorldID       uint8
	Multiplayer   bool
}

type MessageBodyHelloOut struct {
	Magic   [8]byte
	SeqGame uint32
	SeqNet  uint32
}

type MessageBodyWalIn struct {
	Token uint32
	Type  uint8
	Data  []byte
}

type MessageBodyWalQueryIn struct {
	Index uint32
}

type WalItemIn struct {
	To    uint8
	Game  uint8
	GI    uint16
	Flags uint16
	Key   uint32
}

type MessageBodyWalOut struct {
	Type  uint8
	Index uint32
	Data  []byte
}

type WalItemOut struct {
	From       uint8
	To         uint8
	Game       uint8
	GI         uint16
	Flags      uint16
	Key        uint32
	PlayerName [8]byte
}

func ParseMessage(data []byte) (*Message, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("message too short")
	}
	var msg Message
	msg.Seq = binary.BigEndian.Uint32(data[0:4])
	msg.Op = Opcode(data[4])
	msg.Payload = data[5:]
	return &msg, nil
}

func ParseMessageBodyHelloIn(data []byte) (*MessageBodyHelloIn, error) {
	if len(data) < 58 {
		return nil, fmt.Errorf("message body too short for HELLO_IN")
	}
	var body MessageBodyHelloIn
	copy(body.Magic[:], data[0:8])
	copy(body.SessionID[:], data[8:24])
	copy(body.SessionSecret[:], data[24:32])
	copy(body.PlayerID[:], data[32:48])
	copy(body.PlayerName[:], data[48:56])
	body.WorldID = data[56]
	body.Multiplayer = (data[57] != 0)
	return &body, nil
}

func ParseMessageBodyWalIn(data []byte) (*MessageBodyWalIn, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("message body too short for WAL_IN")
	}
	var body MessageBodyWalIn
	body.Token = binary.BigEndian.Uint32(data[0:4])
	body.Type = data[4]
	body.Data = data[5:]
	return &body, nil
}

func ParseWalItemIn(data []byte) (*WalItemIn, error) {
	if len(data) < 9 {
		return nil, fmt.Errorf("WAL item too short")
	}
	var item WalItemIn
	item.To = data[0]
	item.Game = data[1]
	item.GI = binary.BigEndian.Uint16(data[2:4])
	item.Flags = binary.BigEndian.Uint16(data[4:6])
	item.Key = binary.BigEndian.Uint32(data[6:10])
	return &item, nil
}

func (msg *Message) Serialize() []byte {
	data := make([]byte, 5+len(msg.Payload))
	binary.BigEndian.PutUint32(data[0:4], msg.Seq)
	data[4] = byte(msg.Op)
	copy(data[5:], msg.Payload)
	return data
}

func (body *MessageBodyHelloOut) Serialize() []byte {
	data := make([]byte, 16)
	copy(data[0:8], body.Magic[:])
	binary.BigEndian.PutUint32(data[8:12], body.SeqGame)
	binary.BigEndian.PutUint32(data[12:16], body.SeqNet)
	return data
}

func (body *MessageBodyWalOut) Serialize() []byte {
	data := make([]byte, 5+len(body.Data))
	binary.BigEndian.PutUint32(data[0:4], body.Index)
	data[4] = body.Type
	copy(data[5:], body.Data)
	return data
}

func (item *WalItemOut) Serialize() []byte {
	data := make([]byte, 19)
	data[0] = item.From
	data[1] = item.To
	data[2] = item.Game
	binary.BigEndian.PutUint16(data[3:5], item.GI)
	binary.BigEndian.PutUint16(data[5:7], item.Flags)
	binary.BigEndian.PutUint32(data[7:11], item.Key)
	copy(data[11:19], item.PlayerName[:])
	return data
}
