package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/OoTMM/multiplayer/client/internal/app"
	"github.com/OoTMM/multiplayer/client/internal/config"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	procSHChangeNotify = shell32.NewProc("SHChangeNotify")
)

const (
	shcneAssocChanged = 0x08000000
	shcnfIdList       = 0x0000
)

func registerFileAssociation() {
	path, err := os.Executable()
	if err != nil {
		return
	}
	var changed bool

	key, ok, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\.ootmm`, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	changed = changed || !ok
	key.SetStringValue("", "OoTMM.Patch")
	key.Close()

	key, ok, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\OoTMM.Patch`, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	changed = changed || !ok
	key.SetStringValue("", "OoTMM Patch File")
	key.Close()

	key, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\OoTMM.Patch\shell\open\command`, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	changed = changed || !ok
	key.SetStringValue("", "\""+path+"\" \"%1\"")
	key.Close()

	key, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\OoTMM.Patch\DefaultIcon`, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	changed = changed || !ok
	key.SetStringValue("", "\""+path+"\",-2")
	key.Close()

	if changed {
		procSHChangeNotify.Call(shcneAssocChanged, shcnfIdList, 0, 0)
	}
}

func main() {
	registerFileAssociation()

	conf, err := config.ParseConfig(os.Args[1:])
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
		return
	}

	/* Create a new context with signal handling */
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	/* Run the application */
	err = app.Run(ctx, conf)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
