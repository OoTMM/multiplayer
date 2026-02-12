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

func gameReadWalItem(client *Client, data []byte) (*protocol.WalEntry, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("Invalid WAL item data length: %d bytes", len(data))
	}

	walItem := &protocol.WalItem{}
	walEntry := &protocol.WalEntry{
		Type: protocol.WAL_ENTRY_ITEM,
		Item: walItem,
	}
	walItem.PlayerFromUniqueID = client.Info.PlayerUniqueID
	walItem.PlayerFrom = data[0]
	walItem.PlayerTo = data[1]
	walItem.GameID = data[2]
	flags := data[3]
	walItem.Key = binary.BigEndian.Uint32(data[4:8])
	walItem.ItemID = binary.BigEndian.Uint16(data[8:10])

	if (flags & 0x01) != 0 {
		rand.Read(walEntry.UUID[:])
		walEntry.UUID[0] = 0xff
	} else {
		walEntry.UUID[0] = 0x01
		walEntry.UUID[1] = walItem.PlayerFrom
		walEntry.UUID[2] = walItem.PlayerTo
		walEntry.UUID[3] = walItem.GameID
		binary.LittleEndian.PutUint32(walEntry.UUID[4:8], walItem.Key)
	}

	return walEntry, nil
}

func gamePacketWriteWalItem(client *Client, data []byte) error {
	walEntry, err := gameReadWalItem(client, data)
	if err != nil {
		return fmt.Errorf("Failed to read WAL item: %v", err)
	}
	fmt.Println("\nReceived WAL item:")
	fmt.Printf(" * UUID:               %x\n", walEntry.UUID)
	fmt.Printf(" * PlayerFromUniqueID: %016x\n", walEntry.Item.PlayerFromUniqueID)
	fmt.Printf(" * PlayerFrom:         %d\n", walEntry.Item.PlayerFrom)
	fmt.Printf(" * PlayerTo:           %d\n", walEntry.Item.PlayerTo)
	fmt.Printf(" * GameID:             %d\n", walEntry.Item.GameID)
	fmt.Printf(" * Key:                %08x\n", walEntry.Item.Key)
	fmt.Printf(" * ItemID:             %d\n", walEntry.Item.ItemID)
	return client.SendPacketEmpty()
}

func sendPos(client *Client, pos *ClientPos, name []byte) error {
	data := make([]byte, 24)

	binary.BigEndian.PutUint16(data[0:2], pos.Key)
	binary.BigEndian.PutUint16(data[2:4], uint16(client.Info.PlayerUniqueID&0xffff))
	binary.BigEndian.PutUint32(data[4:8], math.Float32bits(pos.X))
	binary.BigEndian.PutUint32(data[8:12], math.Float32bits(pos.Y))
	binary.BigEndian.PutUint32(data[12:16], math.Float32bits(pos.Z))
	copy(data[16:24], name)
	return client.SendPacket(data)
}

func gamePacketExchangePos(client *Client, data []byte) error {
	if len(data) < 14 {
		return fmt.Errorf("Invalid exchange pos data length: %d bytes", len(data))
	}

	/* Store the incoming position */
	client.Pos.Key = binary.BigEndian.Uint16(data[0:2])
	client.Pos.X = math.Float32frombits(binary.BigEndian.Uint32(data[2:6]))
	client.Pos.Y = math.Float32frombits(binary.BigEndian.Uint32(data[6:10]))
	client.Pos.Z = math.Float32frombits(binary.BigEndian.Uint32(data[10:14]))

	/* Echo */
	//err := sendPos(client, &client.Pos, client.Info.NameData[:])
	//if err != nil {
	//	return err
	//}

	/* DEBUG */
	pos2 := &ClientPos{
		Key: 0xffff,
		X:   client.Pos.X + 40.0,
		Y:   client.Pos.Y,
		Z:   client.Pos.Z,
	}

	pos3 := &ClientPos{
		Key: 0xffff,
		X:   client.Pos.X + 60.0,
		Y:   client.Pos.Y,
		Z:   client.Pos.Z,
	}

	pos4 := &ClientPos{
		Key: 0xffff,
		X:   client.Pos.X + 80.0,
		Y:   client.Pos.Y,
		Z:   client.Pos.Z,
	}

	sendPos(client, pos2, client.Info.NameData[:])
	sendPos(client, pos3, client.Info.NameData[:])
	sendPos(client, pos4, client.Info.NameData[:])

	/* Empty packet to signal end of exchange */
	return client.SendPacketEmpty()
}

func gamePacketUnknown(op byte) error {
	fmt.Printf("warn: Received unknown game packet with op: %02x\n", op)
	return nil
}

func GamePacketHandler(client *Client, payload []byte) error {
	if len(payload) == 0 {
		/* Empty packet, reply with empty packet too */
		return client.SendPacketEmpty()
	}

	op := payload[0]
	data := payload[1:]

	switch op {
	case GAME_OP_WRITE_WAL_ITEM:
		return gamePacketWriteWalItem(client, data)
	case GAME_OP_EXCHANGE_POS:
		return gamePacketExchangePos(client, data)
	default:
		return gamePacketUnknown(op)
	}
}
