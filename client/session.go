package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
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
	conn   *IPCConn
	config *Config

	server *protocol.Conn

	ctx    context.Context
	cancel context.CancelFunc

	info SessionInfo
	Pos  GamePos
}

func NewSession(conn net.Conn, config *Config, ctx context.Context) *Session {
	ctx, cancel := context.WithCancel(ctx)
	ipc := NewIPCConn(conn)

	return &Session{
		conn:   ipc,
		config: config,
		ctx:    ctx,
		cancel: cancel,
		info:   SessionInfo{},
		Pos: GamePos{
			Key: 0xffff,
		},
	}
}

func (s *Session) handshake() error {
	/* Handle initial packet */
	pkt, err := s.conn.ReadRaw()
	if err != nil {
		return err
	}
	if len(pkt) < 43 {
		return fmt.Errorf("session handshake: Packet too short: %d bytes", len(pkt))
	}
	pktMagic := string(pkt[0:6])
	if pktMagic != "OoTMM\x00" {
		return fmt.Errorf("session handshake: Invalid magic")
	}

	/* All good, extract infos */
	copy(s.info.NameData[:], pkt[6:14])
	copy(s.info.SessionID[:], pkt[14:30])
	s.info.SessionSecret = binary.BigEndian.Uint32(pkt[30:34])
	s.info.PlayerUniqueID = binary.BigEndian.Uint64(pkt[34:42])
	s.info.PlayerID = pkt[42]

	/* We have an UUID, find the matching session */
	//session := client.app.GetSession(client.Info.SessionID)
	//client.session = session
	//session.AddClient(client)

	/* Respond */
	resp := make([]byte, 6+8)
	copy(resp[0:6], []byte("OoTMM\x00"))
	binary.BigEndian.PutUint32(resp[6:10], s.conn.info.Token)
	binary.BigEndian.PutUint32(resp[10:14], s.conn.info.NextPacketID)
	err = s.conn.WriteRaw(resp)
	if err != nil {
		return fmt.Errorf("New Client: Failed to respond")
	}
	return nil
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
	conn, err := net.Dial("tcp", s.config.ServerAddress)
	if err != nil {
		return fmt.Errorf("Failed to connect to server: %v", err)
	}

	/* Perform handshake */
	protocolConn := protocol.NewConn(conn)
	protocolConn.Start()

	hello := protocol.ClientHello{
		SessionID:      s.info.SessionID,
		SessionSecret:  s.info.SessionSecret,
		WalIndex:       0,
		PlayerName:     s.info.NameData,
		PlayerUniqueID: s.info.PlayerUniqueID,
		PlayerID:       s.info.PlayerID,
	}
	err = protocolConn.WritePacket(protocol.SerializeClientHello(&hello))
	if err != nil {
		conn.Close()
		return fmt.Errorf("Failed to send handshake packet: %v", err)
	}
	s.server = protocolConn
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

	/* Wait for handshake */
	err := s.handshake()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

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
