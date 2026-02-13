package main

import (
	"context"
	"net"
)

type Server struct {
	Conn       net.Conn
	Context    context.Context
	Cancel     context.CancelFunc
	PacketsIn  chan []byte
	PacketsOut chan []byte
}
