package main

import (
	"fmt"

	"github.com/OoTMM/multiplayer/protocol"
)

type Client struct {
	App  *App
	Conn *protocol.Conn
}

func NewClient(app *App, conn *protocol.Conn) *Client {
	return &Client{
		App:  app,
		Conn: conn,
	}
}

func (c *Client) process() {
	/* Process the hello packet */
	helloData, err := c.Conn.ReadPacket()
	if err != nil {
		fmt.Printf("failed to read hello packet: %v\n", err)
		return
	}
	hello, err := protocol.DeserializeClientHello(helloData)
	if err != nil {
		fmt.Printf("failed to read hello packet: %v\n", err)
		return
	}
	sessionInfo := &SessionInfo{}
	copy(sessionInfo.ID[:], hello.SessionID[:])
	sessionInfo.Secret = hello.SessionSecret
	_, err = c.App.JoinSession(c, sessionInfo)
	if err != nil {
		fmt.Printf("failed to join session: %v\n", err)
		return
	}
	fmt.Printf("Client %s joined session %032x\n", string(hello.PlayerName[:]), sessionInfo.ID)
}

func (c *Client) Run() {
	fmt.Printf("Client connected from %s\n", c.Conn.RemoteAddr().String())
	c.Conn.Start()
	go c.process()
	<-c.Conn.Done()
	fmt.Printf("Client disconnected from %s\n", c.Conn.RemoteAddr().String())
}
