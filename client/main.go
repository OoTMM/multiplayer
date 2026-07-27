package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/OoTMM/multiplayer/client/internal/app"
)

func main() {
	/* Create a new context with signal handling */
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	/* Run the application */
	app.Run(ctx)
}
