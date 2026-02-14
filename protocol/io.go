package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

func NetPacketSend(conn net.Conn, data []byte) error {
	fmt.Printf("debug: Send %v\n", data)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(data)))
	_, err := conn.Write(header)
	if err != nil {
		return err
	}
	if len(data) > 0 {
		_, err = conn.Write(data)
		if err != nil {
			return err
		}
	}
	conn.SetWriteDeadline(time.Time{})
	return nil
}

func NetPacketRecv(conn net.Conn) ([]byte, error) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	header := make([]byte, 4)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(header[0:4])
	data := make([]byte, length)
	if length > 0 {
		_, err = io.ReadFull(conn, data)
		if err != nil {
			return nil, err
		}
	}
	conn.SetReadDeadline(time.Time{})
	fmt.Printf("debug: Recv %v\n", data)
	return data, nil
}
