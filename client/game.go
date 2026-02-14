package main

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/OoTMM/multiplayer/protocol"
)

const GAME_OP_WRITE_WAL_ITEM = 0x01
const GAME_OP_EXCHANGE_POS = 0x02

func gameReadWalItem(session *Session, data []byte) (*protocol.WalEntry, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("Invalid WAL item data length: %d bytes", len(data))
	}

	walItem := &protocol.WalItem{}
	walEntry := &protocol.WalEntry{
		ID:   "Test", // TODO: Generate a proper ID
		Type: "ITEM",
		Item: walItem,
	}
	walItem.PlayerID = "test" // TODO: Map player ID to unique ID
	walItem.From = data[0]
	walItem.To = data[1]
	walItem.GameID = data[2]
	//flags := data[3]
	walItem.Key = binary.BigEndian.Uint32(data[4:8])
	walItem.ItemID = binary.BigEndian.Uint16(data[8:10])

	/*
		if (flags & 0x01) != 0 {
			buf := make([]byte, 16)
			_, err := rand.Read(buf)
			if err != nil {
				return nil, fmt.Errorf("Failed to generate random UUID: %v", err)
			}
		} else {
			walEntry.UUID[0] = 0x01
			walEntry.UUID[1] = walItem.PlayerFrom
			walEntry.UUID[2] = walItem.PlayerTo
			walEntry.UUID[3] = walItem.GameID
			binary.LittleEndian.PutUint32(walEntry.UUID[4:8], walItem.Key)
		}
	*/

	return walEntry, nil
}

func gamePacketWriteWalItem(session *Session, data []byte) error {
	walEntry, err := gameReadWalItem(session, data)
	if err != nil {
		return fmt.Errorf("Failed to read WAL item: %v", err)
	}
	fmt.Println("\nReceived WAL item:")
	fmt.Printf(" * ID:                 %s\n", walEntry.ID)
	fmt.Printf(" * PlayerID:           %s\n", walEntry.Item.PlayerID)
	fmt.Printf(" * From:               %d\n", walEntry.Item.From)
	fmt.Printf(" * PlayerTo:           %d\n", walEntry.Item.To)
	fmt.Printf(" * GameID:             %d\n", walEntry.Item.GameID)
	fmt.Printf(" * Key:                %08x\n", walEntry.Item.Key)
	fmt.Printf(" * ItemID:             %d\n", walEntry.Item.ItemID)

	return session.conn.WritePacketEmpty()
}

func sendPos(session *Session, pos *GamePos, name []byte) error {
	data := make([]byte, 24)

	binary.BigEndian.PutUint16(data[0:2], pos.Key)
	binary.BigEndian.PutUint16(data[2:4], uint16(session.info.PlayerUniqueID&0xffff))
	binary.BigEndian.PutUint32(data[4:8], math.Float32bits(pos.X))
	binary.BigEndian.PutUint32(data[8:12], math.Float32bits(pos.Y))
	binary.BigEndian.PutUint32(data[12:16], math.Float32bits(pos.Z))
	copy(data[16:24], name)
	return session.conn.WritePacket(data)
}

func gamePacketExchangePos(session *Session, data []byte) error {
	if len(data) < 14 {
		return fmt.Errorf("Invalid exchange pos data length: %d bytes", len(data))
	}

	/* Store the incoming position */
	session.Pos.Key = binary.BigEndian.Uint16(data[0:2])
	session.Pos.X = math.Float32frombits(binary.BigEndian.Uint32(data[2:6]))
	session.Pos.Y = math.Float32frombits(binary.BigEndian.Uint32(data[6:10]))
	session.Pos.Z = math.Float32frombits(binary.BigEndian.Uint32(data[10:14]))

	/* Echo */
	//err := sendPos(client, &client.Pos, client.Info.NameData[:])
	//if err != nil {
	//	return err
	//}

	/* DEBUG */
	pos2 := &GamePos{
		Key: 0xffff,
		X:   session.Pos.X + 40.0,
		Y:   session.Pos.Y,
		Z:   session.Pos.Z,
	}

	pos3 := &GamePos{
		Key: 0xffff,
		X:   session.Pos.X + 60.0,
		Y:   session.Pos.Y,
		Z:   session.Pos.Z,
	}

	pos4 := &GamePos{
		Key: 0xffff,
		X:   session.Pos.X + 80.0,
		Y:   session.Pos.Y,
		Z:   session.Pos.Z,
	}

	sendPos(session, pos2, session.info.NameData[:])
	sendPos(session, pos3, session.info.NameData[:])
	sendPos(session, pos4, session.info.NameData[:])

	/* Empty packet to signal end of exchange */
	return session.conn.WritePacketEmpty()
}

func gamePacketUnknown(op byte) error {
	fmt.Printf("warn: Received unknown game packet with op: %02x\n", op)
	return nil
}

func GamePacketHandler(session *Session, payload []byte) error {
	if len(payload) == 0 {
		/* Empty packet, reply with empty packet too */
		return session.conn.WritePacketEmpty()
	}

	op := payload[0]
	data := payload[1:]

	switch op {
	case GAME_OP_WRITE_WAL_ITEM:
		return gamePacketWriteWalItem(session, data)
	case GAME_OP_EXCHANGE_POS:
		return gamePacketExchangePos(session, data)
	default:
		return gamePacketUnknown(op)
	}
}
