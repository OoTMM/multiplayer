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
	OpPosition Opcode = 0x05
)

type Message struct {
	Seq     uint32
	Op      Opcode
	Payload []byte
}

type MessageBodyPositionIn struct {
	Key uint16
	X   int16
	Y   int16
	Z   int16
}

type MessageBodyPositionOut struct {
	ID    uint16
	Color uint16
	Name  [8]byte
	Key   uint16
	X     int16
	Y     int16
	Z     int16
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

type MessageBodyWalOut struct {
	Index      uint32
	Type       uint8
	From       uint8
	PlayerName [8]byte
	Data       []byte
}

type WalItem struct {
	To    uint8
	Game  uint8
	GI    uint16
	Flags uint16
	Key   uint32
}

type WalEvent struct {
	EventID uint32
}

type MessageBodyWalQueryIn struct {
	Index uint32
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

func ParseMessageBodyPositionIn(data []byte) (*MessageBodyPositionIn, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("message body too short for POSITION_IN")
	}
	var body MessageBodyPositionIn
	body.Key = binary.BigEndian.Uint16(data[0:2])
	body.X = int16(binary.BigEndian.Uint16(data[2:4]))
	body.Y = int16(binary.BigEndian.Uint16(data[4:6]))
	body.Z = int16(binary.BigEndian.Uint16(data[6:8]))
	return &body, nil
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

func ParseWalItem(data []byte) (*WalItem, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("WAL item too short")
	}
	var item WalItem
	item.To = data[0]
	item.Game = data[1]
	item.GI = binary.BigEndian.Uint16(data[2:4])
	item.Flags = binary.BigEndian.Uint16(data[4:6])
	item.Key = binary.BigEndian.Uint32(data[6:10])
	return &item, nil
}

func ParseWalEvent(data []byte) (*WalEvent, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("WAL event too short")
	}
	var event WalEvent
	event.EventID = binary.BigEndian.Uint32(data[0:4])
	return &event, nil
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
	data := make([]byte, 14+len(body.Data))
	binary.BigEndian.PutUint32(data[0:4], body.Index)
	data[4] = body.Type
	data[5] = body.From
	copy(data[6:14], body.PlayerName[:])
	copy(data[14:], body.Data)
	return data
}

func (item *WalItem) Serialize() []byte {
	data := make([]byte, 10)
	data[0] = item.To
	data[1] = item.Game
	binary.BigEndian.PutUint16(data[2:4], item.GI)
	binary.BigEndian.PutUint16(data[4:6], item.Flags)
	binary.BigEndian.PutUint32(data[6:10], item.Key)
	return data
}

func (event *WalEvent) Serialize() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data[0:4], event.EventID)
	return data
}

func (body *MessageBodyPositionOut) Serialize() []byte {
	data := make([]byte, 20)
	binary.BigEndian.PutUint16(data[0:2], body.ID)
	binary.BigEndian.PutUint16(data[2:4], body.Color)
	copy(data[4:12], body.Name[:])
	binary.BigEndian.PutUint16(data[12:14], body.Key)
	binary.BigEndian.PutUint16(data[14:16], uint16(body.X))
	binary.BigEndian.PutUint16(data[16:18], uint16(body.Y))
	binary.BigEndian.PutUint16(data[18:20], uint16(body.Z))
	return data
}
