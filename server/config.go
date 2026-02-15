package main

import (
	"github.com/spf13/pflag"
)

type Config struct {
	BindAddress string
	BindPort    uint16
}

func ParseConfig() *Config {
	config := &Config{}

	pflag.StringVarP(&config.BindAddress, "bind", "b", "", "Address to bind to (default: all interfaces)")
	pflag.Uint16VarP(&config.BindPort, "port", "p", 12500, "Port to bind to")
	pflag.Parse()

	return config
}
