package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/OoTMM/multiplayer/server/internal/config"
	"github.com/OoTMM/multiplayer/server/internal/events"
	"github.com/OoTMM/multiplayer/server/internal/session"
	"github.com/OoTMM/multiplayer/shared/protocol"
)

type appSession struct {
	session *session.Session
	count   int
}

type App struct {
	ctx        context.Context
	conf       *config.Config
	listener   *net.TCPListener
	sessions   map[[16]byte]*appSession
	sessionsMu sync.Mutex
	sink       events.Sink
}

func Run(ctx context.Context) error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: 14236})
	if err != nil {
		return err
	}

	conf := config.ParseConfig()
	sink := events.NewSink(conf)
	defer sink.Close()

	app := &App{
		ctx:      ctx,
		conf:     conf,
		listener: listener,
		sessions: make(map[[16]byte]*appSession),
		sink:     sink,
	}

	app.run()
	return nil
}

func (app *App) acquireSession(sessionID [16]byte, sessionSecret [8]byte) (*session.Session, error) {
	app.sessionsMu.Lock()
	defer app.sessionsMu.Unlock()

	if appS, ok := app.sessions[sessionID]; ok {
		if appS.session.Secret != sessionSecret {
			return nil, fmt.Errorf("invalid session secret")
		}
		appS.count++
		return appS.session, nil
	}

	session, err := session.OpenSession(app.ctx, app.conf, sessionID, sessionSecret, app.sink)
	if err != nil {
		return nil, fmt.Errorf("failed to open session: %v", err)
	}
	slog.Info("opened session", "sessionId", hex.EncodeToString(sessionID[:]))
	app.sessions[sessionID] = &appSession{
		session: session,
		count:   1,
	}
	return session, nil
}

func (app *App) releaseSession(sessionID [16]byte) {
	app.sessionsMu.Lock()
	defer app.sessionsMu.Unlock()

	if appS, ok := app.sessions[sessionID]; ok {
		appS.count--
		if appS.count <= 0 {
			slog.Info("closing session", "sessionId", hex.EncodeToString(sessionID[:]))
			delete(app.sessions, sessionID)
			appS.session.Close()
		}
	}
}

func (app *App) handleClient(conn *net.TCPConn) {
	defer conn.Close()

	pkt, err := protocol.RecvRawTimeoutDefault(conn)
	if err != nil {
		if app.ctx.Err() == nil {
			slog.Error("failed to receive packet", "error", err)
		}
		return
	}

	if pkt.Op != protocol.OpHello {
		slog.Error("unexpected operation code", "op", pkt.Op)
		return
	}

	hello, err := protocol.ParseClientHello(pkt.Data)
	if err != nil {
		slog.Error("failed to parse hello packet", "error", err)
		return
	}

	slog.Info("received hello", "sessionId", hex.EncodeToString(hello.SessionID[:]), "playerId", hex.EncodeToString(hello.PlayerID[:]), "worldId", hello.WorldID, "walIndex", hello.WalIndex)
	session, err := app.acquireSession(hello.SessionID, hello.SessionSecret)
	if err != nil {
		slog.Error("failed to get session", "error", err)
		return
	}
	defer app.releaseSession(hello.SessionID)

	logger := slog.With("sessionId", hex.EncodeToString(hello.SessionID[:]), "playerId", hex.EncodeToString(hello.PlayerID[:]), "worldId", hello.WorldID, "walIndex", hello.WalIndex)
	logger.Info("player connected")
	session.Join(hello.PlayerID, hello.PlayerName, hello.WorldID, hello.WalIndex, conn)
	logger.Info("player disconnected")
}

func (app *App) run() {
	slog.Info("server started", "address", app.listener.Addr().String())

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
				slog.Error("failed to accept connection", "error", err)
			}
			continue
		}
		conn.SetNoDelay(true)
		wg.Go(func() { app.handleClient(conn) })
	}
	wg.Wait()
}
