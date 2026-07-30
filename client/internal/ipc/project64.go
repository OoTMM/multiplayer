package ipc

import (
	"context"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

type PJ64Conn struct {
	path   string
	handle windows.Handle
}

type PJ64ConnFactory struct {
	path string
}

func listPipes() []string {
	pipes := make([]string, 0)
	entries, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pj64em-ipc.") {
			pipes = append(pipes, `\\.\pipe\`+name)
		}
	}

	return pipes
}

func newPJ64Conn(path string) (*PJ64Conn, error) {
	p, err := windows.UTF16FromString(path)
	if err != nil {
		return nil, err
	}

	h, err := windows.CreateFile(&p[0], windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}

	mode := uint32(windows.PIPE_READMODE_MESSAGE)
	if err := windows.SetNamedPipeHandleState(h, &mode, nil, nil); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}

	return &PJ64Conn{handle: h, path: path}, nil
}

func PollProject64(ctx context.Context) []ConnFactory {
	pipes := listPipes()
	factories := make([]ConnFactory, 0, len(pipes))
	for _, pipe := range pipes {
		factories = append(factories, &PJ64ConnFactory{path: pipe})
	}
	return factories
}

func (conn *PJ64Conn) Close() {
	windows.CancelIoEx(conn.handle, nil)
	windows.CloseHandle(conn.handle)
}

func (conn *PJ64Conn) Read() ([]byte, error) {
	buf := make([]byte, 512)
	var msg []byte
	for {
		var n uint32
		err := windows.ReadFile(conn.handle, buf, &n, nil)
		msg = append(msg, buf[:n]...)
		switch err {
		case nil:
			return msg, nil
		case windows.ERROR_MORE_DATA:
			continue
		default:
			return nil, err
		}
	}
}

func (conn *PJ64Conn) Write(data []byte) error {
	var n uint32
	return windows.WriteFile(conn.handle, data, &n, nil)
}

func (f *PJ64ConnFactory) Open() (Conn, error) {
	conn, err := newPJ64Conn(f.path)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
