package main

import (
	"github.com/spf13/pflag"
)

type Config struct {
	ServerAddress string
}

func ParseConfig() *Config {
	config := &Config{}

	pflag.StringVarP(&config.ServerAddress, "server", "s", "multi.ootmm.com:12500", "Address (and optional port) of the upstream server")
	pflag.Parse()

	return config
}
