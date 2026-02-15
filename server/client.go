package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/OoTMM/multiplayer/protocol"
)

type Client struct {
	App  *App
	Conn *protocol.Conn

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	session *Session
}

func (c *Client) handleRequestWal(data []byte) {
	entry, err := protocol.DeserializeWalEntry(data)
	if err != nil {
		fmt.Printf("Failed to deserialize WAL entry: %v\n", err)
		return
	}
	fmt.Printf("debug: Received WAL entry: %v\n", entry)
	c.session.wal.Append(entry)
}

func (c *Client) handleRequest(packet []byte) {
	fmt.Printf("debug: Received packet %v\n", packet)
	if len(packet) < 1 {
		return
	}
	op := packet[0]
	remain := packet[1:]

	switch op {
	case protocol.NetOpWal:
		c.handleRequestWal(remain)
	default:
		fmt.Printf("warn: Unknown packet op: %02x\n", op)
	}
}

func (c *Client) handleRequests() {
	defer c.cancel()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		packet, err := c.Conn.ReadPacket()
		if err != nil {
			select {
			case <-c.ctx.Done():
			default:
				fmt.Printf("Error reading packet: %v\n", err)
			}
			return
		}

		c.wg.Go(func() { c.handleRequest(packet) })
	}
}

func HandleClient(app *App, conn *protocol.Conn) {
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &Client{
		App:    app,
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}

	/* Register the client */
	app.AddClient(client)
	defer app.RemoveClient(client)

	/* Display client connection info */
	fmt.Printf("Client connected from %s\n", conn.RemoteAddr().String())

	/* Process the hello packet */
	helloData, err := conn.ReadPacket()
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

	session, err := app.JoinSession(client, sessionInfo)
	if err != nil {
		fmt.Printf("failed to join session: %v\n", err)
		return
	}
	client.session = session
	fmt.Printf("Client %s joined session %032x\n", string(hello.PlayerName[:]), sessionInfo.ID)

	/* Subscribe to WAL */
	stream := session.wal.Subscribe(hello.WalIndex, func(index uint32, entry *protocol.WalEntry) {
		fmt.Printf("debug: WAL #%d: %v\n", index, entry)
	})
	defer stream.Close()

	client.wg.Go(client.handleRequests)

	/* Wait for the client to be disconnected */
	<-ctx.Done()
	client.wg.Wait()
	fmt.Printf("Client disconnected from %s\n", conn.RemoteAddr().String())
}

func (c *Client) Shutdown() {
	c.cancel()
	c.Conn.Close()
}
