package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

type UnixConn struct {
	conn *net.UnixConn
}

type UnixConnFactory struct {
	path string
}

func listSockets() []string {
	paths := make([]string, 0)
	dir := getRuntimeDir() + "/n64-ipc"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return paths
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".sock") {
			paths = append(paths, dir+"/"+name)
		}
	}

	return paths
}

func newUnixConn(path string) (*UnixConn, error) {
	fmt.Println("Connecting to", path)
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}

	return &UnixConn{conn: conn}, nil
}

func PollUnix(ctx context.Context) []ConnFactory {
	sockets := listSockets()
	factories := make([]ConnFactory, 0, len(sockets))
	for _, socket := range sockets {
		factories = append(factories, &UnixConnFactory{path: socket})
	}
	return factories
}

func (f *UnixConnFactory) Open() (Conn, error) {
	conn, err := newUnixConn(f.path)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (conn *UnixConn) Close() {
	conn.conn.Close()
}

func (conn *UnixConn) Read() ([]byte, error) {
	var header [4]byte

	// Read the header first
	_, err := io.ReadFull(conn.conn, header[:])
	if err != nil {
		return nil, err
	}

	len := int(binary.LittleEndian.Uint32(header[:]))
	msg := make([]byte, len)
	_, err = io.ReadFull(conn.conn, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (conn *UnixConn) Write(data []byte) error {
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(data)))

	_, err := conn.conn.Write(header[:])
	if err != nil {
		return err
	}

	_, err = conn.conn.Write(data)
	if err != nil {
		return err
	}

	return nil
}
