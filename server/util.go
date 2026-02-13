package main

import (
	"os"
	"runtime"
)

func dataPathPrefix() string {
	envPath := os.Getenv("OOTMM_DATA_PATH")
	if envPath != "" {
		return envPath
	}

	if runtime.GOOS == "windows" {
		return os.Getenv("APPDATA") + "/OoTMM-Server"
	}

	return os.Getenv("HOME") + "/.local/share/ootmm-server"
}

func DataPath(suffix string) string {
	prefix := dataPathPrefix()
	return prefix + "/" + suffix
}
