//go:build !windows

package ipc

import (
	"context"
	"os"
)

var runtimeDir string

func getRuntimeDir() string {
	if runtimeDir == "" {
		runtimeDir = os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			panic("XDG_RUNTIME_DIR is not set")
		}
	}
	return runtimeDir
}

func Poll(ctx context.Context) []ConnFactory {
	return PollUnix(ctx)
}
