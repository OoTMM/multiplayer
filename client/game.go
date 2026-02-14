package main

import (
	"crypto/rand"
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
		Type: protocol.WalTypeItem,
		Item: walItem,
	}
	walItem.PlayerID = session.info.PlayerID
	walItem.From = data[0]
	walItem.To = data[1]
	walItem.GameID = data[2]
	flags := data[3]
	walItem.Key = binary.BigEndian.Uint32(data[4:8])
	walItem.ItemID = binary.BigEndian.Uint16(data[8:10])

	if (flags & 0x01) != 0 {
		buf := make([]byte, 16)
		_, err := rand.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("Failed to generate random UUID: %v", err)
		}
	} else {
		walEntry.ID[0] = 0x01
		walEntry.ID[1] = walItem.From
		walEntry.ID[2] = walItem.To
		walEntry.ID[3] = walItem.GameID
		binary.LittleEndian.PutUint32(walEntry.ID[4:8], walItem.Key)
	}

	return walEntry, nil
}

func gamePacketWriteWalItem(session *Session, data []byte) error {
	walEntry, err := gameReadWalItem(session, data)
	if err != nil {
		return fmt.Errorf("Failed to read WAL item: %v", err)
	}
	fmt.Println("\nReceived WAL item:")
	fmt.Printf(" * ID:                 %032x\n", walEntry.ID)
	fmt.Printf(" * PlayerID:           %032x\n", walEntry.Item.PlayerID)
	fmt.Printf(" * From:               %d\n", walEntry.Item.From)
	fmt.Printf(" * PlayerTo:           %d\n", walEntry.Item.To)
	fmt.Printf(" * GameID:             %d\n", walEntry.Item.GameID)
	fmt.Printf(" * Key:                %08x\n", walEntry.Item.Key)
	fmt.Printf(" * ItemID:             %d\n", walEntry.Item.ItemID)

	/* Serialize */
	data, err = protocol.SerializeWalEntry(walEntry)
	if err != nil {
		return fmt.Errorf("Failed to serialize WAL entry: %v", err)
	}

	/* Add to the send queue (critical) */
	err = session.sendQueue.Add(walEntry.ID, data)
	if err != nil {
		return fmt.Errorf("Failed to add WAL entry to send queue: %v", err)
	}

	/* Reply to the client to confirm reception */
	err = session.conn.WritePacketEmpty()
	if err != nil {
		return fmt.Errorf("Failed to reply to client: %v", err)
	}

	/* Optimistic send (not a fatal error) */

	return nil
}

func sendPos(session *Session, pos *GamePos, name []byte) error {
	data := make([]byte, 24)
	colorIndex := binary.BigEndian.Uint16(session.info.PlayerID[14:16])

	binary.BigEndian.PutUint16(data[0:2], pos.Key)
	binary.BigEndian.PutUint16(data[2:4], colorIndex)
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
