package config

import (
	"os"

	flag "github.com/spf13/pflag"
)

type Config struct {
	DataDir string
	NatsURL string
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

	conf.NatsURL = os.Getenv("OOTMM_SERVER_NATS_URL")

	return &conf
}
