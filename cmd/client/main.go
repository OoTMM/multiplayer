package main

import (
	"fmt"
	"net"

	"github.com/OoTMM/multiplayer/internal/client"
)

func main() {
	config := client.ParseConfig()
	startClient(config)
}

func startClient(config *client.Config) {
	socket, err := net.Listen("tcp", fmt.Sprintf(":%d", config.BindPort))
	if err != nil {
		fmt.Printf("Failed to bind to port: %v\n", err)
		return
	}
	defer socket.Close()
	fmt.Println("OoTMM Multiplayer Client Started")
	for {
		conn, err := socket.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		tcpConn := conn.(*net.TCPConn)
		tcpConn.SetNoDelay(true)

		fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())
		tcpConn.Close()
	}
}
