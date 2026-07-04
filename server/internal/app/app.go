package app

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/OoTMM/multiplayer/shared/protocol"
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

	pkt, err := protocol.RecvRaw(conn)
	if err != nil {
		if app.ctx.Err() == nil {
			fmt.Println("Failed to receive packet:", err)
		}
		return
	}

	if pkt.Op != protocol.OpHello {
		fmt.Printf("Unexpected operation code: %d\n", pkt.Op)
		return
	}

	hello, err := protocol.ParseClientHello(pkt.Data)
	if err != nil {
		fmt.Println("Failed to parse hello packet:", err)
		return
	}

	fmt.Printf("Received hello from player %s (ID: %d, World: %d)\n", hello.PlayerName, hello.PlayerID, hello.WorldID)
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
