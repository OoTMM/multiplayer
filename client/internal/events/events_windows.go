//go:build windows

package events

import (
	"fmt"
	"os"
)

func runDir() string {
	return fmt.Sprintf("%s\\OoTMM", os.Getenv("TMP"))
}
