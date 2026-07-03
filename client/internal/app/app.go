package app

import (
	"context"
	"encoding/binary"
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
	msg, err := conn.Read()
	if err != nil {
		fmt.Println("Failed to read from connection:", err)
		return
	}

	fmt.Printf("Received message: %x\n", msg)

	seq := binary.LittleEndian.Uint32(msg[0:4])
	op := msg[4]
	magic := msg[5:13]

	if op != 0x01 {
		fmt.Printf("unexpected operation code in HELLO: %d\n", op)
		return
	}

	if seq != 0 {
		fmt.Printf("non-zero sequence number in HELLO: %d\n", seq)
		return
	}

	if string(magic) != "OoTMM\x7f\x01\x00" {
		fmt.Printf("unexpected magic in HELLO: %x\n", magic)
		return
	}

	info := game.ReadSessionInfo(msg[13:])
	if info == nil {
		fmt.Println("Failed to read session info from message")
		return
	}

	fmt.Printf("Session Info: %+v\n", info)
}
