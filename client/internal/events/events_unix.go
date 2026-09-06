//go:build !windows

package events

import (
	"fmt"
	"os"
)

func runDir() string {
	return fmt.Sprintf("%s/ootmm", os.Getenv("XDG_RUNTIME_DIR"))
}
