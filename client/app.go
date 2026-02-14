package main

import (
	"context"
	"fmt"
	"net"

	"github.com/natefinch/npipe"
)

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	config   *Config
	listener net.Listener
}

func NewApp(Config *Config, ctx context.Context) *App {
	ctx, cancel := context.WithCancel(ctx)
	return &App{
		ctx:    ctx,
		cancel: cancel,
		config: Config,
	}
}

func (app *App) handleClient(conn net.Conn) {
	defer conn.Close()

	fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())
	//ipc := NewIPCConn(conn)
	session := NewSession(conn, app.config, app.ctx)
	session.Run()
	fmt.Printf("Closed connection from %s\n", conn.RemoteAddr().String())
}

func (app *App) loop() {
	fmt.Printf("OoTMM Multiplayer Client Started\n")

	for {
		/* Check for shutdown */
		select {
		case <-app.ctx.Done():
			return
		default:
		}

		/* Accept connections */
		conn, err := app.listener.Accept()
		if err != nil {
			/* Check if shutdown */
			select {
			case <-app.ctx.Done():
				return
			default:
			}

			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		app.handleClient(conn)
	}
}

func (app *App) Run() {
	/* Create listener */
	listener, err := npipe.Listen("\\\\.\\pipe\\project64-em")
	if err != nil {
		fmt.Printf("Failed to create named pipe: %v\n", err)
		return
	}
	app.listener = listener

	go func() {
		<-app.ctx.Done()
		fmt.Printf("App shutting down...\n")
		listener.Close()
	}()

	app.loop()
}

func (app *App) Shutdown() {
	app.cancel()
}
