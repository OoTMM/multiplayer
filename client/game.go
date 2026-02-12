package main

import "fmt"

func gamePacketWriteWalItem(client *Client, data []byte) error {
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
