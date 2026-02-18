package main

import (
	"encoding/binary"
	"fmt"

	"github.com/OoTMM/multiplayer/shared"
)

func handleWal(session *Session, packet []byte) error {
	index := binary.LittleEndian.Uint32(packet[0:4])
	data := packet[4:]

	entry, err := shared.DeserializeWalEntry(data)
	if err != nil {
		return fmt.Errorf("Failed to deserialize WAL entry: %v", err)
	}

	walCount := session.wal.Count()
	if index != walCount {
		return fmt.Errorf("WAL index mismatch: expected %d, got %d", walCount, index)
	}

	err = session.wal.Append(entry)
	if err != nil {
		return err
	}

	session.sendQueue.Ack(entry.ID)

	fmt.Printf("Received WAL #%d: %032x\n", index, entry.ID)

	return nil
}

func handlePlayerNames(session *Session, packet []byte) error {
	names, err := shared.DeserializePlayerNames(packet)
	if err != nil {
		return fmt.Errorf("Failed to deserialize player names: %v", err)
	}
	for _, name := range names {
		session.names.Add(&name)
	}
	return nil
}

var NetPacketHandlers = map[uint8]func(*Session, []byte) error{
	shared.NetOpWal:         handleWal,
	shared.NetOpPlayerNames: handlePlayerNames,
}

func HandleNetPacket(session *Session, packet []byte) error {
	if len(packet) < 1 {
		return nil
	}

	fmt.Printf("debug: Packet %v\n", packet)

	op := packet[0]
	remain := packet[1:]
	handler := NetPacketHandlers[op]
	if handler == nil {
		fmt.Printf("warn: Unknown packet op: %02x\n", op)
		return nil
	}
	return handler(session, remain)
}
