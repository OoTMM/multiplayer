package app

import (
	"context"
	"fmt"
)

type App struct {
	ctx context.Context
}

func start(app *App) {
	fmt.Println("Client started")
}

func stop(app *App) {
	fmt.Println("Client stopped")
}

func Run(ctx context.Context) {
	app := &App{
		ctx: ctx,
	}
	start(app)
	<-ctx.Done()
	stop(app)
}
