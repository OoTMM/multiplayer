package main

import (
	"fmt"
	"net"
)

func main() {
	config := ParseConfig()
	bind := fmt.Sprintf("%s:%d", config.BindAddress, config.BindPort)
	socket, err := net.Listen("tcp", bind)
	if err != nil {
		fmt.Printf("Failed to bind to port: %v\n", err)
		return
	}
	defer socket.Close()
	fmt.Printf("OoTMM Multiplayer Client Started on %s\n", bind)
	for {
		conn, err := socket.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		go handleClient(conn)
	}
}

type SessionInfo struct {
	UUID [16]byte
}

type Session struct {
	Info SessionInfo
}
