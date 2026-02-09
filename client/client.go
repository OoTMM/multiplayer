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

type Client struct {
	Conn         net.Conn
	Ctx          context.Context
	Cancel       context.CancelFunc
	Token        uint32
	NextPacketID uint32
}

const OP_ENTRY_SEND = 0x01
const OP_ENTRY_GET = 0x02

const ENTRY_ITEM = 0x01

func NewClient(conn net.Conn) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	/* Create a 32 bits random token for this client */
	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	token := binary.BigEndian.Uint32(randBytes[:4])
	nextPacketID := binary.BigEndian.Uint32(randBytes[4:8])

	return &Client{
		Conn:         conn,
		Ctx:          ctx,
		Cancel:       cancel,
		Token:        token,
		NextPacketID: nextPacketID,
	}
}

func clientSendPacket(client *Client, payload []byte) error {
	length := uint16(len(payload))
	if length > 512 {
		return fmt.Errorf("Payload too large: %d bytes", length)
	}
	header := make([]byte, 10)
	binary.BigEndian.PutUint16(header[0:2], length)
	binary.BigEndian.PutUint32(header[2:6], client.Token)
	binary.BigEndian.PutUint32(header[6:10], client.NextPacketID)
	_, err := client.Conn.Write(header)
	if err != nil {
		return fmt.Errorf("Failed to write packet header: %v", err)
	}
	if length > 0 {
		_, err = client.Conn.Write(payload)
		if err != nil {
			return fmt.Errorf("Failed to write packet payload: %v", err)
		}
	}
	client.NextPacketID++
	return nil
}

func clientReadPacket(client *Client) ([]byte, error) {
	header := make([]byte, 10)
	_, err := io.ReadFull(client.Conn, header[0:1])
	if err != nil {
		return nil, fmt.Errorf("Failed to read packet header: %v", err)
	}
	client.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, err = io.ReadFull(client.Conn, header[1:10])
	if err != nil {
		return nil, fmt.Errorf("Failed to read packet header: %v", err)
	}
	length := binary.BigEndian.Uint16(header[0:2])
	if length > 512 {
		return nil, fmt.Errorf("Invalid packet length: %d", length)
	}
	token := binary.BigEndian.Uint32(header[2:6])
	packetID := binary.BigEndian.Uint32(header[6:10])
	if token != client.Token {
		return nil, fmt.Errorf("Invalid token in packet: %08x", token)
	}
	if packetID != client.NextPacketID {
		return nil, fmt.Errorf("Invalid packet ID: expected %d, got %d", client.NextPacketID, packetID)
	}
	payload := make([]byte, length)
	if length > 0 {
		_, err = io.ReadFull(client.Conn, payload)
		if err != nil {
			return nil, fmt.Errorf("Failed to read packet payload: %v", err)
		}
	}
	client.Conn.SetReadDeadline(time.Time{})
	client.NextPacketID++
	return payload, nil
}

func clientHandshake(client *Client) error {
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
	info := make([]byte, 8)
	binary.BigEndian.PutUint32(info[0:4], client.Token)
	binary.BigEndian.PutUint32(info[4:8], client.NextPacketID)
	_, err = client.Conn.Write(info)
	if err != nil {
		return fmt.Errorf("New Client: Failed to send handshake info: %v", err)
	}
	return nil
}

func clientSendPacketEmpty(client *Client) error {
	return clientSendPacket(client, []byte{})
}

func processPacketEntrySend(client *Client, payload []byte) error {
	/* TODO: Implement this */
	fmt.Printf("debug: Received ENTRY_SEND packet with payload: %x\n", payload[1:])
	return clientSendPacketEmpty(client)
}

func processPacketUnknown(client *Client, payload []byte) error {
	fmt.Printf("warn: Ignoring unknown packet type %d\n", payload[0])
	return clientSendPacketEmpty(client)
}

func processPacket(client *Client, payload []byte) error {
	op := payload[0]

	switch op {
	case OP_ENTRY_SEND:
		return processPacketEntrySend(client, payload)
	default:
		return processPacketUnknown(client, payload)
	}
}

func clientLoop(client *Client) {
	for {
		payload, err := clientReadPacket(client)
		if err != nil {
			fmt.Printf("Error reading packet: %v\n", err)
			return
		}

		err = processPacket(client, payload)
		if err != nil {
			fmt.Printf("Error processing packet: %v\n", err)
			return
		}
	}
}

func handleClient(conn net.Conn) {
	/* Basic setup */
	defer conn.Close()
	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetNoDelay(true)

	/* Create the client */
	client := NewClient(conn)

	/* Wait for magic */
	err := clientHandshake(client)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())
	clientLoop(client)
}
