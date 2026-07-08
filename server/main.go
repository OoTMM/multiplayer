package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/OoTMM/multiplayer/server/internal/app"
)

func main() {
	/* Config */
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	/* Create a new context with signal handling */
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	/* Run the application */
	app.Run(ctx)
}
