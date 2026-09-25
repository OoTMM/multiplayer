package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

func SendRaw(conn net.Conn, pkt *Packet) error {
	/* Size check */
	size := len(pkt.Data)
	if size > 65535 {
		return fmt.Errorf("packet size exceeds maximum allowed size of 65535 bytes")
	}

	/* Create packet */
	data := make([]byte, 3+size)
	binary.LittleEndian.PutUint16(data[0:2], uint16(size))
	data[2] = byte(pkt.Op)
	copy(data[3:], pkt.Data)

	/* Send */
	_, err := conn.Write(data)
	return err
}

func SendRawTimeout(conn net.Conn, pkt *Packet, timeout time.Duration) error {
	conn.SetWriteDeadline(time.Now().Add(timeout))
	defer conn.SetWriteDeadline(time.Time{})
	return SendRaw(conn, pkt)
}

func SendRawTimeoutDefault(conn net.Conn, pkt *Packet) error {
	return SendRawTimeout(conn, pkt, 10*time.Second)
}

func RecvRaw(conn net.Conn) (*Packet, error) {
	header := make([]byte, 3)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint16(header[0:2])
	data := make([]byte, size)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return nil, err
	}
	return &Packet{
		Op:   Opcode(header[2]),
		Data: data,
	}, nil
}

func RecvRawTimeout(conn net.Conn, timeout time.Duration) (*Packet, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	defer conn.SetReadDeadline(time.Time{})
	return RecvRaw(conn)
}

func RecvRawTimeoutDefault(conn net.Conn) (*Packet, error) {
	return RecvRawTimeout(conn, 10*time.Second)
}
