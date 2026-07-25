package app

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/OoTMM/multiplayer/server/internal/session"
	"github.com/OoTMM/multiplayer/shared/protocol"
)

type App struct {
	ctx        context.Context
	listener   *net.TCPListener
	sessions   map[[16]byte]*session.Session
	sessionsMu sync.Mutex
}

func Run(ctx context.Context) error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: 14236})
	if err != nil {
		return err
	}

	app := &App{
		ctx:      ctx,
		listener: listener,
		sessions: make(map[[16]byte]*session.Session),
	}

	app.run()
	return nil
}

func (app *App) getSession(sessionID [16]byte, sessionSecret [8]byte) (*session.Session, error) {
	app.sessionsMu.Lock()
	defer app.sessionsMu.Unlock()

	if s, ok := app.sessions[sessionID]; ok {
		if s.Secret != sessionSecret {
			return nil, fmt.Errorf("invalid session secret")
		}
		return s, nil
	}

	session, err := session.OpenSession(app.ctx, sessionID, sessionSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to open session: %v", err)
	}
	app.sessions[sessionID] = session
	return session, nil
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
	session, err := app.getSession(hello.SessionID, hello.SessionSecret)
	if err != nil {
		fmt.Println("Failed to get session:", err)
		return
	}

	session.Join(hello.PlayerID, hello.PlayerName, hello.WorldID, hello.WalIndex, conn)
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
