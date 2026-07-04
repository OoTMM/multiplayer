package app

import (
	"context"
	"fmt"
	"net"
	"sync"
)

type App struct {
	ctx      context.Context
	listener *net.TCPListener
}

func Run(ctx context.Context) error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: 14236})
	if err != nil {
		return err
	}

	app := &App{
		ctx:      ctx,
		listener: listener,
	}

	app.run()
	return nil
}

func (app *App) handleClient(conn *net.TCPConn) {
	defer conn.Close()
}

func (app *App) run() {
	fmt.Println("Server started on", app.listener.Addr().String())

	wg := sync.WaitGroup{}
	wg.Go(func() {
		<-app.ctx.Done()
		app.listener.Close()
	})

	for {
		if app.ctx.Err() != nil {
			break
		}

		conn, err := app.listener.AcceptTCP()
		if err != nil {
			if app.ctx.Err() == nil {
				fmt.Println("Failed to accept connection:", err)
			}
			continue
		}
		conn.SetNoDelay(true)
		wg.Go(func() { app.handleClient(conn) })
	}
}
