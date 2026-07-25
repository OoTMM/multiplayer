package config

import (
	"os"

	flag "github.com/spf13/pflag"
)

type Config struct {
	DataDir        string
	UpstreamServer string
	UpstreamPort   uint16
}

func ParseConfig() *Config {
	var defaultDataDir string
	confDir, err := os.UserConfigDir()
	if err != nil {
		defaultDataDir = "./data"
	} else {
		defaultDataDir = confDir + "/OoTMM/client"
	}

	var conf Config
	fs := flag.CommandLine
	fs.StringVarP(&conf.DataDir, "data", "d", defaultDataDir, "Path to the data directory")
	fs.StringVarP(&conf.UpstreamServer, "server", "s", "multi.ootmm.com", "Upstream server address")
	fs.Uint16VarP(&conf.UpstreamPort, "port", "p", 14236, "Upstream server port")
	fs.Parse(os.Args[1:])

	return &conf
}
