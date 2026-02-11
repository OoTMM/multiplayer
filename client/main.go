package main

import (
	"fmt"

	"github.com/natefinch/npipe"
)

func main() {
	//config := ParseConfig()
	pipe, err := npipe.Listen("\\\\.\\pipe\\project64-em")
	if err != nil {
		fmt.Printf("Failed to create named pipe: %v\n", err)
		return
	}
	defer pipe.Close()
	fmt.Printf("OoTMM Multiplayer Client Started\n")
	for {
		conn, err := pipe.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		go handleClient(conn)
	}
}
