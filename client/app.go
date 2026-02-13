package main

import (
	"fmt"
	"net"

	"github.com/natefinch/npipe"
)

type App struct {
	config *Config
	conn   net.Listener

	session *Session
}

func NewApp(Config *Config) *App {
	return &App{
		config: Config,
	}
}

func (app *App) handleClient(conn net.Conn) {
	ipc := NewIPCConn(conn)
	session := NewSession(ipc)
	app.session = session
	session.Run()
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

		fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())
		app.handleClient(conn)
		conn.Close()
		fmt.Printf("Closed connection from %s\n", conn.RemoteAddr().String())
	}
}
