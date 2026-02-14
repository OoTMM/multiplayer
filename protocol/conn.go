package protocol

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type Conn struct {
	conn       net.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	packetsIn  chan []byte
	packetsOut chan []byte
}

func (c *Conn) onError(err error) {
	select {
	case <-c.ctx.Done():
	default:
		fmt.Printf("Conn error: %v\n", err)
		c.cancel()
	}
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
		case <-c.ctx.Done():
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
		case <-c.ctx.Done():
			return
		case <-time.After(5 * time.Second):
			c.packetsOut <- []byte{}
		}
	}
}

func newProtocolConn(tcp net.Conn) *Conn {
	if tcp, ok := tcp.(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		tcp.Close()
	}()

	conn := &Conn{
		conn:       tcp,
		ctx:        ctx,
		cancel:     cancel,
		packetsIn:  make(chan []byte, 64),
		packetsOut: make(chan []byte, 64),
	}

	conn.wg.Go(conn.readLoop)
	conn.wg.Go(conn.writeLoop)
	conn.wg.Go(conn.heartbeat)

	return conn
}

func (c *Conn) ReadPacket() ([]byte, error) {
	for {
		select {
		case <-c.ctx.Done():
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
	case <-c.ctx.Done():
		return fmt.Errorf("WritePacket: Client disconnected")
	case c.packetsOut <- data:
		return nil
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) Close() {
	c.cancel()
	c.wg.Wait()
}

func DialProtocol(address string) (*Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to server: %v", err)
	}
	return newProtocolConn(conn), nil
}

type ConnListener struct {
	listener net.Listener
}

func ListenProtocol(address string) (*ConnListener, error) {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on address %s: %v", address, err)
	}
	return &ConnListener{listener: ln}, nil
}

func (l *ConnListener) Accept() (*Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("Failed to accept connection: %v", err)
	}
	return newProtocolConn(conn), nil
}

func (l *ConnListener) Close() {
	l.listener.Close()
}
