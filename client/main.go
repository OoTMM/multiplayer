package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/natefinch/npipe"
)

func main() {
	config := ParseConfig()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	RunServer(config, ctx)
}

func RunServer(config *Config, ctx context.Context) {
	listener, err := npipe.Listen("\\\\.\\pipe\\project64-em")
	if err != nil {
		fmt.Printf("Failed to create named pipe: %v\n", err)
		return
	}
	defer listener.Close()

	fmt.Printf("Listening on %s\n", listener.Addr())

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			fmt.Printf("Failed to accept: %v\n", err)
			continue
		}

		err = RunSession(conn, config, ctx)
		if err != nil {
			fmt.Printf("Session error: %v\n", err)
		}
	}
}
