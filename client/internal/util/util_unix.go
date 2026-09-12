//go:build !windows

package util

import (
	"fmt"
	"os"
	"syscall"
)

func RunDir() string {
	return fmt.Sprintf("%s/ootmm", os.Getenv("XDG_RUNTIME_DIR"))
}

func StartDetachedProcess(path string, args []string) error {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	attr := &os.ProcAttr{
		Dir:   "",
		Files: []*os.File{devNull, devNull, devNull},
		Sys: &syscall.SysProcAttr{
			Setsid: true,
			Noctty: true,
		},
	}
	process, err := os.StartProcess(path, args, attr)
	if err != nil {
		return err
	}
	process.Release()
	return nil
}
