package app

import (
	"context"
	"fmt"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/config"
	"github.com/OoTMM/multiplayer/client/internal/game"
	"github.com/OoTMM/multiplayer/client/internal/ipc"
	"github.com/fatih/color"
)

type App struct {
	info *game.Info
	conf *config.Config
	ctx  context.Context
}

func Run(ctx context.Context, conf *config.Config) error {
	info, err := game.ExtractGameInfo(conf)
	if err != nil {
		return fmt.Errorf("failed to extract game info: %w", err)
	}

	app := &App{
		info: info,
		conf: conf,
		ctx:  ctx,
	}

	app.displayInfo()
	app.loop()
	return nil
}

func (app *App) displayInfo() {
	var modeStr string

	switch app.info.Mode {
	case game.InfoModeSingle:
		modeStr = "Single Player"
	case game.InfoModeCoop:
		modeStr = "Co-op"
	case game.InfoModeMulti:
		modeStr = "Multiplayer"
	default:
		modeStr = "Unknown"
	}

	star := color.BlueString(" * ")
	fmt.Printf("%s%s%s\n", star, color.HiWhiteString(fmt.Sprintf("%-12s", "Session ID: ")), color.HiCyanString("%x", app.info.SessionID))
	fmt.Printf("%s%s%s\n", star, color.HiWhiteString(fmt.Sprintf("%-12s", "World ID: ")), color.HiCyanString("%x", app.info.WorldID))
	fmt.Printf("%s%s%s\n", star, color.HiWhiteString(fmt.Sprintf("%-12s", "Mode: ")), color.HiCyanString("%s", modeStr))
	fmt.Printf("\n")
}

func (app *App) hello(conn ipc.Conn) (*ipc.MessageBodyHelloIn, error) {
	raw, err := conn.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read from connection: %w", err)
	}

	msg, err := ipc.ParseMessage(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	if msg.Op != ipc.OpHello {
		return nil, fmt.Errorf("unexpected operation code: %d", msg.Op)
	}

	if msg.Seq != 0 {
		return nil, fmt.Errorf("unexpected sequence number: %d", msg.Seq)
	}

	hello, err := ipc.ParseMessageBodyHelloIn(msg.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HELLO_IN message body: %w", err)
	}

	if string(hello.Magic[:]) != "OoTMM\x7f\x01\x00" {
		return nil, fmt.Errorf("unexpected magic: %x", hello.Magic)
	}

	return hello, nil
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
		hello, err := app.hello(conn)
		if err != nil {
			fmt.Println("Failed to read HELLO message:", err)
			conn.Close()
			continue
		}
		return conn, hello
	}

	return nil, nil
}

func (app *App) loop() {
	fmt.Println("Waiting for Project64 to connect...")
	for {
		if app.ctx.Err() != nil {
			break
		}
		conn, hello := app.poll()
		if conn != nil && hello != nil {
			game.Run(app.ctx, app.conf, app.info, conn, hello)
		} else {
			select {
			case <-app.ctx.Done():
				break
			case <-time.After(time.Second):
			}
		}
	}
}
