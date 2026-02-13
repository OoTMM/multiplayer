package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := ParseConfig()
	app := NewApp(config, ctx)
	app.Run()
}
