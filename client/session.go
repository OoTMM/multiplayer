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

	"github.com/OoTMM/multiplayer/protocol"
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
	wal       *protocol.WAL
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

func sessionHandshake(conn IPCConn) (*SessionInfo, error) {
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

func StartSession(conn net.Conn, config *Config, ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ipc := NewIPCConn(conn)
	defer ipc.Close()

	info, err := sessionHandshake(*ipc)
	if err != nil {
		fmt.Printf("Failed to perform handshake: %v\n", err)
		return
	}

	path := SessionPath(info.SessionID)
	err = os.MkdirAll(path, 0700)
	if err != nil {
		fmt.Printf("Failed to create session directory: %v\n", err)
		return
	}

	wal, err := protocol.OpenWAL(path + "/wal.bin")
	if err != nil {
		fmt.Printf("Failed to open WAL: %v\n", err)
		return
	}
	defer wal.Close()

	/* Create the server link */
	server := CreateServerLink(config.ServerAddress, info, wal)
	defer server.Close()

	/* Create the send queue */
	queuePath := fmt.Sprintf("%s/queue.%d.bin", path, info.WorldID)
	sendQueue, err := OpenSendQueue(queuePath)
	if err != nil {
		fmt.Printf("Failed to open send queue: %v\n", err)
		return
	}
	defer sendQueue.Close()

	session := &Session{
		conn:      ipc,
		config:    config,
		ctx:       ctx,
		wal:       wal,
		server:    server,
		sendQueue: sendQueue,
		cancel:    cancel,
		info:      info,
		Pos: GamePos{
			Key: 0xffff,
		},
	}

	session.Run()
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
				s.server.WritePacket(protocol.SerializeMessage(protocol.NetOpWal, entry))
			}
		}
	}
}

func (s *Session) handleNetPacketWal(packet []byte) error {
	index := binary.LittleEndian.Uint32(packet[0:4])
	data := packet[4:]

	entry, err := protocol.DeserializeWalEntry(data)
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
	case protocol.NetOpWal:
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

func (s *Session) loop() {
	var wg sync.WaitGroup
	wg.Go(s.ipcLoop)
	wg.Go(s.sendQueueLoop)
	wg.Go(s.serverLoop)
	wg.Wait()
}

func (s *Session) Run() {
	go func() {
		<-s.ctx.Done()
		s.conn.Close()
		s.server.Close()
	}()

	fmt.Println("\nClient connected!")
	fmt.Printf(" * Name:           %s\n", string(bytes.Trim(s.info.NameData[:], "\x00")))
	fmt.Printf(" * SessionID:      %032x\n", s.info.SessionID)
	fmt.Printf(" * SessionSecret:  %08x\n", s.info.SessionSecret)
	fmt.Printf(" * PlayerID:	   %032x\n", s.info.PlayerID)
	fmt.Printf(" * WorldID:        %d\n", s.info.WorldID)

	s.loop()

	fmt.Println("Terminating client")
}

func (s *Session) Shutdown() {
	s.cancel()
	s.server.Close()
	s.conn.Close()
}
