package app

import (
	"context"
	"fmt"

	"github.com/OoTMM/multiplayer/client/internal/emulators"
)

type App struct {
	ctx      context.Context
	detector *emulators.Detector
}

func (app *App) start() {
	fmt.Println("Client started")
}

func (app *App) stop() {
	app.detector.Close()
	fmt.Println("Client stopped")
}

func Run(ctx context.Context) {
	app := &App{
		ctx:      ctx,
		detector: emulators.NewDetector(ctx),
	}

	app.start()
	<-ctx.Done()
	app.stop()
}
