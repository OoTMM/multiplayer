package main

import (
	"os"

	"github.com/OoTMM/multiplayer/client/internal/app"
	"github.com/OoTMM/multiplayer/client/internal/daemon"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--daemon" {
		daemon.Run()
	} else {
		app.Start()
	}
}
