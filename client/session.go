package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/shared"
)

type SessionInfo struct {
	SessionID     [16]byte
	SessionSecret uint32
	PlayerID      [16]byte
	WorldID       uint8
	NameData      [8]byte
}

type GamePos struct {
	Key uint16
	X   float32
	Y   float32
	Z   float32
}

type Session struct {
	config *Config

	conn      *IPCConn
	wal       *shared.WAL
	server    *ServerLink
	sendQueue *SendQueue

	ctx    context.Context
	cancel context.CancelFunc

	info *SessionInfo
	Pos  GamePos
}

func SessionPath(sessionID [16]byte) string {
	str := fmt.Sprintf("sessions/%02x/%02x/%028x", sessionID[0], sessionID[1], sessionID[2:])
	return DataPath(str)
}

func handshake(conn IPCConn) (*SessionInfo, error) {
	/* Handle initial packet */
	pkt, err := conn.ReadRaw()
	if err != nil {
		return nil, err
	}
	if len(pkt) < 51 {
		return nil, fmt.Errorf("session handshake: Packet too short: %d bytes", len(pkt))
	}
	pktMagic := string(pkt[0:6])
	if pktMagic != "OoTMM\x00" {
		return nil, fmt.Errorf("session handshake: Invalid magic")
	}

	/* All good, extract infos */
	info := &SessionInfo{}

	copy(info.NameData[:], pkt[6:14])
	copy(info.SessionID[:], pkt[14:30])
	info.SessionSecret = binary.BigEndian.Uint32(pkt[30:34])
	copy(info.PlayerID[:], pkt[34:50])
	info.WorldID = pkt[50]

	/* Respond */
	resp := make([]byte, 6+8)
	copy(resp[0:6], []byte("OoTMM\x00"))
	binary.BigEndian.PutUint32(resp[6:10], conn.info.Token)
	binary.BigEndian.PutUint32(resp[10:14], conn.info.NextPacketID)
	err = conn.WriteRaw(resp)
	if err != nil {
		return nil, fmt.Errorf("New Client: Failed to respond")
	}

	return info, nil
}

func (s *Session) ipcLoop() {
	for {
		payload, err := s.conn.ReadPacket()
		if err != nil {
			fmt.Printf("Error reading packet: %v\n", err)
			s.Shutdown()
			return
		}

		err = GamePacketHandler(s, payload)
		if err != nil {
			fmt.Printf("Error processing packet: %v\n", err)
			s.Shutdown()
			return
		}
	}
}

func (s *Session) sendQueueLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(10 * time.Second):
			data := s.sendQueue.Pending()
			for _, entry := range data {
				s.server.WritePacket(shared.SerializeMessage(shared.NetOpWal, entry))
			}
		}
	}
}

func (s *Session) handleNetPacketWal(packet []byte) error {
	index := binary.LittleEndian.Uint32(packet[0:4])
	data := packet[4:]

	entry, err := shared.DeserializeWalEntry(data)
	if err != nil {
		return fmt.Errorf("Failed to deserialize WAL entry: %v", err)
	}

	walCount := s.wal.Count()
	if index != walCount {
		return fmt.Errorf("WAL index mismatch: expected %d, got %d", walCount, index)
	}

	err = s.wal.Append(entry)
	if err != nil {
		return err
	}

	s.sendQueue.Ack(entry.ID)

	fmt.Printf("Received WAL #%d: %032x\n", index, entry.ID)

	return nil
}

func (s *Session) handleNetPacket(packet []byte) error {
	if len(packet) < 1 {
		return nil
	}

	fmt.Printf("debug: Packet %v\n", packet)

	op := packet[0]
	remain := packet[1:]

	switch op {
	case shared.NetOpWal:
		return s.handleNetPacketWal(remain)
	default:
		fmt.Printf("warn: Unknown packet op: %02x\n", op)
		return nil
	}
}

func (s *Session) serverLoop() {
	defer s.cancel()

	for {
		select {
		case <-s.ctx.Done():
			return
		case packet := <-s.server.Packets():
			err := s.handleNetPacket(packet)
			if err != nil {
				fmt.Printf("Error handling server packet: %v\n", err)
				return
			}
		}
	}
}

func (s *Session) Shutdown() {
	s.cancel()
	s.server.Close()
	s.conn.Close()
}

func RunSession(conn net.Conn, config *Config, ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	/* Create IPC */
	ipc := NewIPCConn(conn)
	defer ipc.Close()

	/* Handshake to get session info */
	info, err := handshake(*ipc)
	if err != nil {
		return err
	}

	/* Get and prepare session directory */
	path := SessionPath(info.SessionID)
	err = os.MkdirAll(path, 0700)
	if err != nil {
		return err
	}

	/* Open WAL */
	wal, err := shared.OpenWAL(path + "/wal.bin")
	if err != nil {
		return err
	}
	defer wal.Close()

	/* Create the server link */
	server := CreateServerLink(config.ServerAddress, info, wal)
	defer server.Close()

	/* Create the send queue */
	queuePath := fmt.Sprintf("%s/queue.%d.bin", path, info.WorldID)
	sendQueue, err := OpenSendQueue(queuePath)
	if err != nil {
		return err
	}
	defer sendQueue.Close()

	session := &Session{
		config:    config,
		conn:      ipc,
		wal:       wal,
		server:    server,
		sendQueue: sendQueue,
		ctx:       ctx,
		cancel:    cancel,
		info:      info,
	}

	/* TODO: review this */
	go func() {
		<-ctx.Done()
		ipc.Close()
		server.Close()
	}()

	fmt.Println("\nClient connected!")
	fmt.Printf(" * Name:           %s\n", string(bytes.Trim(info.NameData[:], "\x00")))
	fmt.Printf(" * SessionID:      %032x\n", info.SessionID)
	fmt.Printf(" * SessionSecret:  %08x\n", info.SessionSecret)
	fmt.Printf(" * PlayerID:	   %032x\n", info.PlayerID)
	fmt.Printf(" * WorldID:        %d\n", info.WorldID)

	var wg sync.WaitGroup
	wg.Go(session.ipcLoop)
	wg.Go(session.sendQueueLoop)
	wg.Go(session.serverLoop)
	wg.Wait()

	fmt.Println("Terminating client")

	return nil
}
