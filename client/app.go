package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/natefinch/npipe"
)

type App struct {
	config        *Config
	conn          net.Listener
	sessionsMutex sync.Mutex
	sessions      map[[16]byte]*Session
}

func NewApp(Config *Config) *App {
	return &App{
		config:   Config,
		sessions: make(map[[16]byte]*Session),
	}
}

func (app *App) Run() {
	pipe, err := npipe.Listen("\\\\.\\pipe\\project64-em")
	if err != nil {
		fmt.Printf("Failed to create named pipe: %v\n", err)
		return
	}
	defer pipe.Close()
	app.conn = pipe
	fmt.Printf("OoTMM Multiplayer Client Started\n")
	for {
		conn, err := pipe.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		go app.onClient(conn)
	}
}

func (app *App) GetSession(uuid [16]byte) *Session {
	app.sessionsMutex.Lock()
	defer app.sessionsMutex.Unlock()
	session := app.sessions[uuid]
	if session == nil {
		session = NewSession(uuid)
		app.sessions[uuid] = session
		session.Start()
	}
	return session
}

func (app *App) onClient(conn net.Conn) {
	client := NewClient(app, conn)
	client.Start()
}
