package main

import (
	"fmt"
	"sync"

	"github.com/OoTMM/multiplayer/shared"
)

type App struct {
	config   *Config
	listener *shared.ConnListener

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
	var err error
	var storedInfo *SessionInfo

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
		storedInfo, err = LoadSessionInfo(info.ID)
		if err != nil {
			return nil, fmt.Errorf("could not load session secret: %v", err)
		}
		if storedInfo != nil && storedInfo.Secret != info.Secret {
			return nil, fmt.Errorf("invalid session secret")
		}
		session, err = NewSession(app, *info, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %v", err)
		}
		app.sessions[info.ID] = session
	}

	return session, nil
}

func (app *App) AddClient(client *Client) {
	app.clientsMutex.Lock()
	app.clients[client] = true
	app.clientsMutex.Unlock()
}

func (app *App) RemoveClient(client *Client) {
	app.clientsMutex.Lock()
	delete(app.clients, client)
	app.clientsMutex.Unlock()
}

func (app *App) Run() {
	bindAddr := fmt.Sprintf("%s:%d", app.config.BindAddress, app.config.BindPort)
	listener, err := shared.ListenProtocol(bindAddr)
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
		go HandleClient(app, conn)
	}
}
