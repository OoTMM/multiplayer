//go:build windows

package ipc

import (
	"context"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

type PJ64Conn struct {
	path       string
	handle     windows.Handle
	readEvent  windows.Handle
	writeEvent windows.Handle
}

type PJ64ConnFactory struct {
	path string
}

func listPipes() []string {
	pipes := make([]string, 0)
	entries, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		return pipes
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

	h, err := windows.CreateFile(&p[0], windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	}

	mode := uint32(windows.PIPE_READMODE_MESSAGE)
	if err := windows.SetNamedPipeHandleState(h, &mode, nil, nil); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}

	readEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(h)
		return nil, err
	}

	writeEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(readEvent)
		windows.CloseHandle(h)
		return nil, err
	}

	return &PJ64Conn{handle: h, path: path, readEvent: readEvent, writeEvent: writeEvent}, nil
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
	windows.CloseHandle(conn.readEvent)
	windows.CloseHandle(conn.writeEvent)
}

func waitOverlapped(handle windows.Handle, event windows.Handle, startErr error, ov *windows.Overlapped) (uint32, error) {
	if startErr != nil && startErr != windows.ERROR_IO_PENDING {
		return 0, startErr
	}

	if startErr == windows.ERROR_IO_PENDING {
		if _, err := windows.WaitForSingleObject(event, windows.INFINITE); err != nil {
			windows.CancelIoEx(handle, ov)
			return 0, err
		}
	}

	var n uint32
	err := windows.GetOverlappedResult(handle, ov, &n, false)
	return n, err
}

func (conn *PJ64Conn) Read() ([]byte, error) {
	buf := make([]byte, 512)
	var msg []byte

	for {
		var ov windows.Overlapped
		ov.HEvent = conn.readEvent
		windows.ResetEvent(conn.readEvent)

		var n uint32
		startErr := windows.ReadFile(conn.handle, buf, &n, &ov)
		n, err := waitOverlapped(conn.handle, conn.readEvent, startErr, &ov)
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
	var ov windows.Overlapped
	ov.HEvent = conn.writeEvent
	windows.ResetEvent(conn.writeEvent)

	var n uint32
	startErr := windows.WriteFile(conn.handle, data, &n, &ov)
	_, err := waitOverlapped(conn.handle, conn.writeEvent, startErr, &ov)
	return err
}

func (f *PJ64ConnFactory) Open() (Conn, error) {
	conn, err := newPJ64Conn(f.path)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
