package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
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
}

func NewSession(app *App, info SessionInfo, firstClient *Client) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{
		app:      app,
		info:     info,
		clients:  make(map[*Client]bool),
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		initChan: make(chan struct{}),
		initErr:  nil,
	}

	session.clients[firstClient] = true
	session.clientsCount = 1

	go session.run()

	return session
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
