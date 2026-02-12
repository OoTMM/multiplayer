package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/OoTMM/multiplayer/protocol"
)

func gameReadWalItem(client *Client, data []byte) (*protocol.WalEntry, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("Invalid WAL item data length: %d bytes", len(data))
	}

	walItem := &protocol.WalItem{}
	walEntry := &protocol.WalEntry{
		Type: protocol.WAL_ENTRY_ITEM,
		Item: walItem,
	}
	walItem.PlayerFromUniqueID = client.PlayerUniqueID
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
	case OP_WRITE_WAL_ITEM:
		return gamePacketWriteWalItem(client, data)
	default:
		return gamePacketUnknown(op)
	}
}
