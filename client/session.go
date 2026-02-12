package main

import "sync"

type Session struct {
	UUID        [16]byte
	clients     map[*Client]bool
	clientMutex sync.RWMutex
}

func NewSession(UUID [16]byte) *Session {
	return &Session{
		UUID:    UUID,
		clients: make(map[*Client]bool),
	}
}

func (s *Session) AddClient(client *Client) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	s.clients[client] = true
}

func (s *Session) RemoveClient(client *Client) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	delete(s.clients, client)
}

func (s *Session) Start() {

}
