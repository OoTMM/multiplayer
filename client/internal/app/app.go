package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/OoTMM/multiplayer/client/internal/game"
	"github.com/OoTMM/multiplayer/client/internal/ipc"
)

type App struct {
	ctx context.Context
}

func (app *App) start() {
	fmt.Println("Client started")
}

func (app *App) stop() {
	fmt.Println("Client stopped")
}

func Run(ctx context.Context) {
	var wg sync.WaitGroup

	app := &App{
		ctx: ctx,
	}

	app.start()
	wg.Go(func() { ipc.ServePJ64(ctx, app.handle) })
	<-ctx.Done()
	wg.Wait()
	app.stop()
}

func (app *App) handle(conn ipc.Conn) {
	raw, err := conn.Read()
	if err != nil {
		fmt.Println("Failed to read from connection:", err)
		return
	}

	fmt.Printf("Received message: %x\n", raw)
	msg, err := ipc.ParseMessage(raw)
	if err != nil {
		fmt.Println("Failed to parse message:", err)
		return
	}

	if msg.Op != ipc.OpHello {
		fmt.Printf("Unexpected operation code: %d\n", msg.Op)
		return
	}

	if msg.Seq != 0 {
		fmt.Printf("Unexpected sequence number: %d\n", msg.Seq)
		return
	}

	hello, err := ipc.ParseMessageBodyHelloIn(msg.Payload)
	if err != nil {
		fmt.Println("Failed to parse HELLO_IN message body:", err)
		return
	}

	if string(hello.Magic[:]) != "OoTMM\x7f\x01\x00" {
		fmt.Printf("Unexpected magic: %x\n", hello.Magic)
		return
	}

	session := game.OpenSession(hello.SessionID, hello.SessionSecret)
	session.ProcessPlayer(app.ctx, hello, conn)
}
