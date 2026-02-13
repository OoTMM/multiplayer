package protocol

import (
	"context"
	"fmt"
	"net"
	"time"
)

type Conn struct {
	conn       net.Conn
	context    context.Context
	cancel     context.CancelFunc
	packetsIn  chan []byte
	packetsOut chan []byte
}

func NewConn(conn net.Conn) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	return &Conn{
		conn:       conn,
		context:    ctx,
		cancel:     cancel,
		packetsIn:  make(chan []byte, 64),
		packetsOut: make(chan []byte, 64),
	}
}

func (c *Conn) onError(err error) {
	fmt.Printf("Conn error: %v\n", err)
	c.cancel()
}

func (c *Conn) readLoop() {
	for {
		data, err := NetPacketRecv(c.conn)
		if err != nil {
			c.onError(fmt.Errorf("Failed to receive packet: %v", err))
			return
		}
		c.packetsIn <- data
	}
}

func (c *Conn) writeLoop() {
	for {
		select {
		case <-c.context.Done():
			return
		case data := <-c.packetsOut:
			err := NetPacketSend(c.conn, data)
			if err != nil {
				c.onError(fmt.Errorf("Failed to send packet: %v", err))
				return
			}
		}
	}
}

func (c *Conn) heartbeat() {
	for {
		select {
		case <-c.context.Done():
			return
		case <-time.After(5 * time.Second):
			c.packetsOut <- []byte{}
		}
	}
}

func (c *Conn) ReadPacket() ([]byte, error) {
	for {
		select {
		case <-c.context.Done():
			return nil, fmt.Errorf("Client disconnected")
		case data := <-c.packetsIn:
			if len(data) == 0 {
				/* Heartbeat packet, ignore */
				continue
			}
			return data, nil
		}
	}
}

func (c *Conn) WritePacket(data []byte) error {
	select {
	case <-c.context.Done():
		return fmt.Errorf("WritePacket: Client disconnected")
	case c.packetsOut <- data:
		return nil
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) Start() {
	/* Set TCP_NODELAY */
	if tcpConn, ok := c.conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	go c.heartbeat()
	go c.readLoop()
	go c.writeLoop()
}

func (c *Conn) Done() <-chan struct{} {
	return c.context.Done()
}

func (c *Conn) Close() {
	c.cancel()
	c.conn.Close()
}
