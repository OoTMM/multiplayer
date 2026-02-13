package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/OoTMM/multiplayer/protocol"
)

type App struct {
	config   *Config
	listener net.Listener

	clients      map[*Client]bool
	clientsMutex sync.RWMutex

	sessions      map[[16]byte]*Session
	sessionsMutex sync.Mutex
}

func NewApp(config *Config) *App {
	return &App{
		config:   config,
		clients:  make(map[*Client]bool),
		sessions: make(map[[16]byte]*Session),
	}
}

func (app *App) JoinSession(client *Client, info *SessionInfo) (*Session, error) {
	app.sessionsMutex.Lock()
	defer app.sessionsMutex.Unlock()
	session := app.sessions[info.ID]
	if session != nil {
		existingInfo := session.Info()
		if existingInfo.Secret != info.Secret {
			return nil, fmt.Errorf("invalid session secret")
		}
		session.AddClient(client)
	} else {
		storedInfo, err := LoadSessionInfo(info.ID)
		if err != nil {
			return nil, fmt.Errorf("could not load session secret")
		}
		if storedInfo != nil && storedInfo.Secret != info.Secret {
			return nil, fmt.Errorf("invalid session secret")
		}
		session := NewSession(app, *info, client)
		app.sessions[info.ID] = session
	}

	return session, nil
}

func (app *App) handleClient(tcpConn net.Conn) {
	conn := protocol.NewConn(tcpConn)
	client := NewClient(app, conn)

	app.clientsMutex.Lock()
	app.clients[client] = true
	app.clientsMutex.Unlock()

	client.Run()

	app.clientsMutex.Lock()
	delete(app.clients, client)
	app.clientsMutex.Unlock()
}

func (app *App) Run() {
	bindAddr := fmt.Sprintf("%s:%d", app.config.BindAddress, app.config.BindPort)
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		fmt.Printf("failed to bind to address %s: %v\n", bindAddr, err)
		return
	}
	app.listener = listener
	defer listener.Close()

	fmt.Printf("OoTMM Multiplayer Server Started on %s\n", bindAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("failed to accept connection: %v\n", err)
			continue
		}
		go app.handleClient(conn)
	}
}
