package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/OoTMM/multiplayer/protocol"
)

func SessionPath(sessionID [16]byte) string {
	localPath := fmt.Sprintf("sessions/%02x/%02x/%028x", sessionID[0], sessionID[1], sessionID[2:])
	return DataPath(localPath)
}

func LoadSessionInfo(sessionID [16]byte) (*SessionInfo, error) {
	path := SessionPath(sessionID) + "/secret.bin"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) != 4 {
		return nil, fmt.Errorf("invalid session secret file size: %d", len(data))
	}
	secret := binary.LittleEndian.Uint32(data)
	return &SessionInfo{
		ID:     sessionID,
		Secret: secret,
	}, nil
}

type SessionInfo struct {
	ID     [16]byte
	Secret uint32
}

type Session struct {
	app  *App
	info SessionInfo

	clients      map[*Client]bool
	clientMutex  sync.RWMutex
	clientsCount int

	ctx          context.Context
	cancel       context.CancelFunc
	shutdownOnce sync.Once
	done         chan struct{}

	initChan chan struct{}
	initErr  error

	wal   *protocol.WAL
	names *protocol.PlayerNamesStore
}

func NewSession(app *App, info SessionInfo, firstClient *Client) (*Session, error) {
	ctx, cancel := context.WithCancel(context.Background())

	path := SessionPath(info.ID)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create session directory: %v", err)
	}

	wal, err := protocol.OpenWAL(path + "/wal.bin")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open WAL: %v", err)
	}

	names, err := protocol.OpenPlayerNamesStore(path + "/names.bin")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open player names store: %v", err)
	}

	names.Add(&protocol.PlayerName{
		ID:   firstClient.ID,
		Name: firstClient.Name,
	})

	session := &Session{
		app:      app,
		info:     info,
		clients:  make(map[*Client]bool),
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		initChan: make(chan struct{}),
		initErr:  nil,
		wal:      wal,
		names:    names,
	}

	session.clients[firstClient] = true
	session.clientsCount = 1

	go session.run()

	return session, nil
}

func (s *Session) Info() SessionInfo {
	return s.info
}

func (s *Session) init() error {
	return nil
}

func (s *Session) run() {
	/* Initialize and signal */
	err := s.init()
	if err != nil {
		s.initErr = err
		close(s.initChan)
		return
	}
	close(s.initChan)

	fmt.Printf("Session started: %032x\n", s.info.ID)
}

func (s *Session) AddClient(client *Client) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	s.clients[client] = true
	s.clientsCount++

	s.names.Add(&protocol.PlayerName{
		ID:   client.ID,
		Name: client.Name,
	})
}

func (s *Session) RemoveClient(client *Client) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	delete(s.clients, client)
	s.clientsCount--
}

func (s *Session) WaitReady() error {
	<-s.initChan
	return s.initErr
}
