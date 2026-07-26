package config

import (
	"os"

	flag "github.com/spf13/pflag"
)

type Config struct {
	DataDir string
}

func ParseConfig() *Config {
	defaultDataDir := os.Getenv("OOTMM_SERVER_DATA_DIR")
	if defaultDataDir == "" {
		defaultDataDir = "./data/server"
	}

	var conf Config
	fs := flag.CommandLine
	fs.StringVarP(&conf.DataDir, "data", "d", defaultDataDir, "Path to the data directory")
	fs.Parse(os.Args[1:])

	return &conf
}
