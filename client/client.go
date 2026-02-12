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
	app            *App
	session        *Session
	Conn           net.Conn
	Ctx            context.Context
	Cancel         context.CancelFunc
	Token          uint32
	NextPacketID   uint32
	SessionID      [16]byte
	SessionSecret  uint32
	PlayerUniqueID uint64
	PlayerID       uint8
	TeamID         uint8
}

const OP_WRITE_WAL_ITEM = 0x01

func NewClient(app *App, conn net.Conn) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	/* Set a read timeout of 30 seconds */
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	/* Create a 32 bits random token for this client */
	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	token := binary.BigEndian.Uint32(randBytes[:4])
	nextPacketID := binary.BigEndian.Uint32(randBytes[4:8])

	return &Client{
		app:          app,
		Conn:         conn,
		Ctx:          ctx,
		Cancel:       cancel,
		Token:        token,
		NextPacketID: nextPacketID,
	}
}

func (client *Client) SendRaw(data []byte) error {
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(data)))
	_, err := client.Conn.Write(header)
	if err != nil {
		return fmt.Errorf("Failed to write packet header: %v", err)
	}
	if len(data) > 0 {
		_, err = client.Conn.Write(data)
		if err != nil {
			return fmt.Errorf("Failed to write packet data: %v", err)
		}
	}
	return nil
}

func (client *Client) RecvRaw() ([]byte, error) {
	client.Conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	header := make([]byte, 2)
	_, err := io.ReadFull(client.Conn, header)
	if err != nil {
		return nil, fmt.Errorf("Failed to read packet header: %v", err)
	}
	length := binary.BigEndian.Uint16(header[0:2])
	if length > 512 {
		return nil, fmt.Errorf("Invalid packet length: %d", length)
	}
	data := make([]byte, length)
	if length > 0 {
		_, err = io.ReadFull(client.Conn, data)
		if err != nil {
			return nil, fmt.Errorf("Failed to read packet data: %v", err)
		}
	}
	return data, nil
}

func (client *Client) SendPacket(payload []byte) error {
	length := uint16(len(payload))
	lengthTotal := length + 8
	if lengthTotal > 512 {
		return fmt.Errorf("Payload too large: %d bytes", length)
	}
	header := make([]byte, 10)
	binary.BigEndian.PutUint16(header[0:2], lengthTotal)
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

func (client *Client) RecvPacket() ([]byte, error) {
	raw, err := client.RecvRaw()
	if err != nil {
		return nil, err
	}
	if len(raw) < 8 {
		return nil, fmt.Errorf("Packet too short: %d bytes", len(raw))
	}
	token := binary.BigEndian.Uint32(raw[0:4])
	packetID := binary.BigEndian.Uint32(raw[4:8])
	if token != client.Token {
		return nil, fmt.Errorf("Invalid packet token: %08x", token)
	}
	if packetID != client.NextPacketID {
		return nil, fmt.Errorf("Invalid packet ID: expected %d, got %d", client.NextPacketID, packetID)
	}
	client.NextPacketID++
	return raw[8:], nil
}

func (client *Client) Handshake() error {
	/* Handle initial packet */
	pkt, err := client.RecvRaw()
	if err != nil {
		return err
	}
	if len(pkt) < 36 {
		return fmt.Errorf("New Client: Packet too short: %d bytes", len(pkt))
	}
	pktMagic := string(pkt[0:6])
	if pktMagic != "OoTMM\x00" {
		return fmt.Errorf("New Client: Invalid magic")
	}

	/* All good, extract infos */
	copy(client.SessionID[:], pkt[6:22])
	client.SessionSecret = binary.BigEndian.Uint32(pkt[22:26])
	client.PlayerUniqueID = binary.BigEndian.Uint64(pkt[26:34])
	client.PlayerID = pkt[34]
	client.TeamID = pkt[35]

	/* We have an UUID, find the matching session */
	session := client.app.GetSession(client.SessionID)
	client.session = session
	session.AddClient(client)

	/* Respond */
	resp := make([]byte, 6+8)
	copy(resp[0:6], []byte("OoTMM\x00"))
	binary.BigEndian.PutUint32(resp[6:10], client.Token)
	binary.BigEndian.PutUint32(resp[10:14], client.NextPacketID)
	err = client.SendRaw(resp)
	if err != nil {
		return fmt.Errorf("New Client: Failed to respond")
	}
	return nil
}

func (client *Client) SendPacketEmpty() error {
	return client.SendPacket([]byte{})
}

func (client *Client) Loop() {
	for {
		payload, err := client.RecvPacket()
		if err != nil {
			fmt.Printf("Error reading packet: %v\n", err)
			return
		}

		err = GamePacketHandler(client, payload)
		if err != nil {
			fmt.Printf("Error processing packet: %v\n", err)
			return
		}
	}
}

func (client *Client) Start() {
	/* Basic setup */
	defer client.Conn.Close()

	/* Wait for magic */
	err := client.Handshake()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	fmt.Println("\nClient connected!")
	fmt.Printf(" * SessionID:      %x\n", client.SessionID)
	fmt.Printf(" * SessionSecret:  %08x\n", client.SessionSecret)
	fmt.Printf(" * PlayerUniqueID: %016x\n", client.PlayerUniqueID)
	fmt.Printf(" * PlayerID:       %d\n", client.PlayerID)
	fmt.Printf(" * TeamID:         %d\n", client.TeamID)

	client.Loop()
	fmt.Println("Terminating client")
}
