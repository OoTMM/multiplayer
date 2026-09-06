//go:build windows

package util

import (
	"fmt"
	"os"
	"syscall"
)

func RunDir() string {
	return fmt.Sprintf("%s\\OoTMM", os.Getenv("TMP"))
}

func StartDetachedProcess(path string, args []string) error {
	attr := &os.ProcAttr{
		Dir:   "",
		Files: []*os.File{nil, nil, nil},
		Sys: &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		},
	}
	process, err := os.StartProcess(path, args, attr)
	if err != nil {
		return err
	}
	process.Release()
	return nil
}
