package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type IPCConnInfo struct {
	Token        uint32
	NextPacketID uint32
}

type IPCConn struct {
	conn   net.Conn
	ctx    context.Context
	cancel context.CancelFunc
	info   IPCConnInfo
}

func NewIPCConn(conn net.Conn) *IPCConn {
	ctx, cancel := context.WithCancel(context.Background())

	/* Create a 32 bits random token for this client */
	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	token := binary.BigEndian.Uint32(randBytes[:4])
	nextPacketID := binary.BigEndian.Uint32(randBytes[4:8])

	ipc := &IPCConn{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
		info: IPCConnInfo{
			Token:        token,
			NextPacketID: nextPacketID,
		},
	}

	return ipc
}

func (c *IPCConn) Info() IPCConnInfo {
	return c.info
}

func (c *IPCConn) ReadRaw() ([]byte, error) {
	header := make([]byte, 2)
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, err := io.ReadFull(c.conn, header)
	if err != nil {
		return nil, fmt.Errorf("failed to read packet header: %v", err)
	}
	length := binary.BigEndian.Uint16(header)
	data := make([]byte, length)
	_, err = io.ReadFull(c.conn, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read packet data: %v", err)
	}
	c.conn.SetReadDeadline(time.Time{})
	return data, nil
}

func (c *IPCConn) ReadPacket() ([]byte, error) {
	data, err := c.ReadRaw()
	if err != nil {
		return nil, err
	}
	if len(data) < 8 {
		return nil, fmt.Errorf("invalid packet data length: %d bytes", len(data))
	}
	token := binary.BigEndian.Uint32(data[0:4])
	packetID := binary.BigEndian.Uint32(data[4:8])
	if token != c.info.Token {
		return nil, fmt.Errorf("invalid packet token: %08x", token)
	}
	if packetID != c.info.NextPacketID {
		return nil, fmt.Errorf("invalid packet ID: expected %08x, got %08x", c.info.NextPacketID, packetID)
	}
	c.info.NextPacketID++
	return data[8:], nil
}

func (c *IPCConn) ReadPacketNonEmpty() ([]byte, error) {
	for {
		data, err := c.ReadPacket()
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			/* Empty packet, ignore */
			continue
		}
		return data, nil
	}
}

func (c *IPCConn) WriteRaw(data []byte) error {
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header, uint16(len(data)))
	_, err := c.conn.Write(header)
	if err != nil {
		return fmt.Errorf("failed to write packet header: %v", err)
	}
	if len(data) > 0 {
		_, err = c.conn.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write packet data: %v", err)
		}
	}
	return nil
}

func (c *IPCConn) WritePacket(payload []byte) error {
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], c.info.Token)
	binary.BigEndian.PutUint32(header[4:8], c.info.NextPacketID)
	c.info.NextPacketID++
	return c.WriteRaw(append(header, payload...))
}

func (c *IPCConn) WritePacketEmpty() error {
	return c.WritePacket([]byte{})
}
