package protocol

import (
	"encoding/binary"
	"io"
	"net"
	"time"
)

func SendRaw(conn net.Conn, pkt *Packet) error {
	size := uint16(len(pkt.Data))
	header := make([]byte, 3)
	binary.LittleEndian.PutUint16(header[0:2], size)
	header[2] = byte(pkt.Op)

	_, err := conn.Write(header)
	if err != nil {
		return err
	}
	_, err = conn.Write(pkt.Data)
	return err
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
