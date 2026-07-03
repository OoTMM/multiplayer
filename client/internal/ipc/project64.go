package ipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

type PJ64Conn struct {
	path string
	pipe net.Conn
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

func newPJ64Conn(path string, pipe net.Conn) *PJ64Conn {
	return &PJ64Conn{
		path: path,
		pipe: pipe,
	}
}

func ServePJ64(ctx context.Context, cb func(conn RawConn)) error {
	var mu sync.Mutex
	var wg sync.WaitGroup

	active := make(map[string]struct{})
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-ticker.C:
			pipes := listPipes()
			mu.Lock()
			for _, pipe := range pipes {
				if _, ok := active[pipe]; ok {
					continue
				}
				fmt.Println("Found new pipe:", pipe)
				p, err := winio.DialPipeContext(ctx, pipe)
				if err != nil {
					fmt.Println("Failed to connect to pipe:", err)
					continue
				}
				c := newPJ64Conn(pipe, p)
				active[pipe] = struct{}{}
				wg.Go(func() {
					fmt.Println("There is a new pipe!")
					cb(c)
					<-ctx.Done()
					c.Close()
					mu.Lock()
					delete(active, c.path)
					mu.Unlock()
				})
			}
			mu.Unlock()
		}
	}
}

func (conn *PJ64Conn) Close() error {
	return conn.pipe.Close()
}

func (conn *PJ64Conn) Read() ([]byte, error) {
	return nil, nil
}

func (conn *PJ64Conn) Write(data []byte) error {
	_, err := conn.pipe.Write(data)
	return err
}
