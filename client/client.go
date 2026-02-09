package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"time"
)

type Client struct {
	Conn   net.Conn
	Ctx    context.Context
	Cancel context.CancelFunc
}

func NewClient(conn net.Conn) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		Conn:   conn,
		Ctx:    ctx,
		Cancel: cancel,
	}
}

func processClientMagic(client *Client) error {
	magic := make([]byte, 6)
	_, err := io.ReadFull(client.Conn, magic)
	if err != nil {
		return fmt.Errorf("New Client: Failed to read magic: %v", err)
	}
	if string(magic) != "OoTMM\x00" {
		return fmt.Errorf("New Client: Invalid magic: %s", string(magic))
	}
	_, err = client.Conn.Write([]byte("OoTMM\x00"))
	if err != nil {
		return fmt.Errorf("New Client: Failed to respond to magic: %v", err)
	}
	return nil
}

func readPacket(client *Client) ([]byte, error) {
	header := make([]byte, 6)
	_, err := io.ReadFull(client.Conn, header[0:1])
	if err != nil {
		return nil, fmt.Errorf("Failed to read packet header: %v", err)
	}
	client.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, err = io.ReadFull(client.Conn, header[1:6])
	if err != nil {
		return nil, fmt.Errorf("Failed to read packet header: %v", err)
	}
	length := binary.BigEndian.Uint16(header[0:2])
	checksum := binary.BigEndian.Uint32(header[2:6])
	if length > 512 {
		return nil, fmt.Errorf("Invalid packet length: %d", length)
	}
	payload := make([]byte, length)
	_, err = io.ReadFull(client.Conn, payload)
	if err != nil {
		return nil, fmt.Errorf("Failed to read packet payload: %v", err)
	}
	client.Conn.SetReadDeadline(time.Time{})
	crc32 := crc32.ChecksumIEEE(payload)
	if crc32 != checksum {
		return nil, fmt.Errorf("Invalid packet checksum: expected 0x%x, got 0x%x", checksum, crc32)
	}
	return payload, nil
}

func handleClient(conn net.Conn) {
	/* Basic setup */
	defer conn.Close()
	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetNoDelay(true)

	/* Create the client */
	client := NewClient(conn)

	/* Wait for magic */
	err := processClientMagic(client)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	/* Print connection info */
	fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())

	for {
		payload, err := readPacket(client)
		if err != nil {
			fmt.Printf("Error reading packet from %s: %v\n", conn.RemoteAddr().String(), err)
			return
		}

		/* Dump packet */
		fmt.Printf("Received %d byte packet from %s\n", len(payload), conn.RemoteAddr().String())
	}
}
