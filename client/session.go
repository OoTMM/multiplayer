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
	SessionID      [16]byte
	SessionSecret  uint32
	PlayerUniqueID uint64
	PlayerID       uint8
	NameData       [8]byte
}

type GamePos struct {
	Key uint16
	X   float32
	Y   float32
	Z   float32
}

type Session struct {
	config *Config

	conn   *IPCConn
	server *protocol.Conn
	wal    *protocol.WAL

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
	if len(pkt) < 43 {
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
	info.PlayerUniqueID = binary.BigEndian.Uint64(pkt[34:42])
	info.PlayerID = pkt[42]

	/* We have an UUID, find the matching session */
	//session := client.app.GetSession(client.Info.SessionID)
	//client.session = session
	//session.AddClient(client)

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

	wal, err := protocol.OpenWAL(path + "/wal.jsonl")
	if err != nil {
		fmt.Printf("Failed to open WAL: %v\n", err)
		return
	}
	defer wal.Close()

	session := &Session{
		conn:   ipc,
		config: config,
		ctx:    ctx,
		wal:    wal,
		cancel: cancel,
		info:   info,
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

func (s *Session) serverConnect() error {
	/* Establish */
	conn, err := protocol.DialProtocol(s.config.ServerAddress)
	if err != nil {
		return fmt.Errorf("Failed to connect to server: %v", err)
	}

	hello := protocol.ClientHello{
		SessionID:      s.info.SessionID,
		SessionSecret:  s.info.SessionSecret,
		WalIndex:       0,
		PlayerName:     s.info.NameData,
		PlayerUniqueID: s.info.PlayerUniqueID,
		PlayerID:       s.info.PlayerID,
	}
	err = conn.WritePacket(protocol.SerializeClientHello(&hello))
	if err != nil {
		conn.Close()
		return fmt.Errorf("Failed to send handshake packet: %v", err)
	}
	s.server = conn
	return nil
}

func (s *Session) serverPerform() {
	for {
		select {
		case <-s.ctx.Done():
			s.server.Close()
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Session) serverLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		err := s.serverConnect()
		if err != nil {
			fmt.Printf("Error connecting to server: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		s.serverPerform()
	}
}

func (s *Session) loop() {
	var wg sync.WaitGroup
	wg.Go(s.ipcLoop)
	wg.Go(s.serverLoop)
	wg.Wait()
}

func (s *Session) Run() {
	go func() {
		<-s.ctx.Done()
		s.conn.Close()
	}()

	fmt.Println("\nClient connected!")
	fmt.Printf(" * Name:           %s\n", string(bytes.Trim(s.info.NameData[:], "\x00")))
	fmt.Printf(" * SessionID:      %x\n", s.info.SessionID)
	fmt.Printf(" * SessionSecret:  %08x\n", s.info.SessionSecret)
	fmt.Printf(" * PlayerUniqueID: %016x\n", s.info.PlayerUniqueID)
	fmt.Printf(" * PlayerID:       %d\n", s.info.PlayerID)

	s.loop()

	fmt.Println("Terminating client")
}

func (s *Session) Shutdown() {
	s.cancel()
}
