package main

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/natefinch/npipe"
)

type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	config   *Config
	listener net.Listener

	session *Session
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
	ipc := NewIPCConn(conn)
	session := NewSession(app, ipc)
	app.session = session
	session.Run()
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

		fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())
		app.handleClient(conn)
		conn.Close()
		fmt.Printf("Closed connection from %s\n", conn.RemoteAddr().String())
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

	/* Loop */
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.loop()
	}()
	<-app.ctx.Done()

	fmt.Printf("App shutting down...\n")

	/* Close the listener */
	listener.Close()

	/* Wait for loop to exit */
	app.wg.Wait()

	/* Close the session */
	if app.session != nil {
		app.session.Shutdown()
		app.session = nil
	}
}

func (app *App) Shutdown() {
	app.cancel()
}
