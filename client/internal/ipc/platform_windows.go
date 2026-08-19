//go:build windows

package ipc

import (
	"context"
	"os"
)

var runtimeDir string

func getRuntimeDir() string {
	if runtimeDir == "" {
		tmp := os.Getenv("LOCALAPPDATA")
		if tmp == "" {
			panic("LOCALAPPDATA is not set")
		}
		tmp += "\\Temp"
		runtimeDir = tmp
	}
	return runtimeDir
}

func Poll(ctx context.Context) []ConnFactory {
	pj64 := PollProject64(ctx)
	unix := PollUnix(ctx)
	return append(pj64, unix...)
}
