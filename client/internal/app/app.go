package app

import (
	"context"
	"fmt"
	"sync"

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

func (app *App) handle(conn ipc.RawConn) {
}
