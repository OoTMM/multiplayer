package main

import (
	"github.com/spf13/pflag"
)

type Config struct {
	BindPort      uint16
	BindAddress   string
	ServerAddress string
}

func ParseConfig() *Config {
	config := &Config{}

	pflag.Uint16VarP(&config.BindPort, "port", "p", 55630, "Port to bind the client to")
	pflag.StringVarP(&config.BindAddress, "bind", "b", "127.0.0.1", "Address to bind the client to")
	pflag.StringVarP(&config.ServerAddress, "server", "s", "multi.ootmm.com:12500", "Address (and optional port) of the upstream server")
	pflag.Parse()

	return config
}
