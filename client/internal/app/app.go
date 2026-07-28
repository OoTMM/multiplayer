package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/config"
	"github.com/OoTMM/multiplayer/client/internal/game"
	"github.com/OoTMM/multiplayer/client/internal/ipc"
)

type App struct {
	conf *config.Config
	ctx  context.Context
}

func Run(ctx context.Context) {
	conf, err := config.ParseConfig(os.Args[1:])
	if err != nil {
		fmt.Println("Failed to parse config:", err)
		return
	}

	app := &App{
		conf: conf,
		ctx:  ctx,
	}

	app.loop()
}

func (app *App) hello(conn ipc.Conn) *ipc.MessageBodyHelloIn {
	raw, err := conn.Read()
	if err != nil {
		fmt.Println("Failed to read from connection:", err)
		return nil
	}

	fmt.Printf("Received message: %x\n", raw)
	msg, err := ipc.ParseMessage(raw)
	if err != nil {
		fmt.Println("Failed to parse message:", err)
		return nil
	}

	if msg.Op != ipc.OpHello {
		fmt.Printf("Unexpected operation code: %d\n", msg.Op)
		return nil
	}

	if msg.Seq != 0 {
		fmt.Printf("Unexpected sequence number: %d\n", msg.Seq)
		return nil
	}

	hello, err := ipc.ParseMessageBodyHelloIn(msg.Payload)
	if err != nil {
		fmt.Println("Failed to parse HELLO_IN message body:", err)
		return nil
	}

	if string(hello.Magic[:]) != "OoTMM\x7f\x01\x00" {
		fmt.Printf("Unexpected magic: %x\n", hello.Magic)
		return nil
	}

	return hello
}

func (app *App) poll() (ipc.Conn, *ipc.MessageBodyHelloIn) {
	/* Do one pass of polling */
	factories := ipc.PollProject64(app.ctx)
	for _, factory := range factories {
		conn, err := factory.Open()
		if err != nil {
			fmt.Println("Failed to open connection:", err)
			continue
		}
		hello := app.hello(conn)
		if hello != nil {
			return conn, hello
		}
		conn.Close()
	}

	return nil, nil
}

func (app *App) loop() {
	fmt.Println("Client started")
	for {
		if app.ctx.Err() != nil {
			break
		}
		conn, hello := app.poll()
		if conn != nil && hello != nil {
			game.Run(app.ctx, app.conf, conn, hello)
		} else {
			select {
			case <-app.ctx.Done():
				break
			case <-time.After(time.Second):
			}
		}
	}
	fmt.Println("Client stopped")
}
