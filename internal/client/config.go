package client

import (
	"github.com/spf13/pflag"
)

type Config struct {
	BindPort      uint16
	ServerAddress string
}

func ParseConfig() *Config {
	config := &Config{}

	pflag.Uint16VarP(&config.BindPort, "port", "p", 12345, "Port to bind the client to")
	pflag.StringVarP(&config.ServerAddress, "server", "s", "multi.ootmm.com:12345", "Address of the server to connect to")
	pflag.Parse()

	return config
}
