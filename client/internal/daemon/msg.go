package daemon

import (
	"encoding/binary"
	"io"
	"net"
)

func SendMsg(conn net.Conn, data []byte) error {
	var header [4]byte
	size := uint32(len(data))
	binary.LittleEndian.PutUint32(header[:], size)
	_, err := conn.Write(header[:])
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func RecvMsg(conn net.Conn) ([]byte, error) {
	var header [4]byte
	_, err := io.ReadFull(conn, header[:])
	if err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[:])
	data := make([]byte, size)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
