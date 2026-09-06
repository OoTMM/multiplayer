package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/util"
)

var daemonSocketPath = fmt.Sprintf("%s/internal.sock", util.RunDir())

type daemon struct {
	ctx         context.Context
	cancel      context.CancelFunc
	listener    *net.UnixListener
	mu          sync.Mutex
	clientCount int
}

/* Bind to the unix socket, remove it if stale */
func Run() {
	os.MkdirAll(util.RunDir(), 0o700)

	addr, err := net.ResolveUnixAddr("unix", daemonSocketPath)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		panic(err)
	}

	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	daemon := &daemon{
		ctx:         ctx,
		cancel:      cancel,
		listener:    listener,
		clientCount: 0,
	}
	daemon.run()
}

func (d *daemon) watchdog() {
	for d.ctx.Err() == nil {
		select {
		case <-d.ctx.Done():
			return
		case <-time.After(5 * time.Second):
			d.mu.Lock()
			if d.clientCount == 0 {
				d.cancel()
			}
			d.mu.Unlock()
		}
	}
}

func (d *daemon) run() {
	/* Close the listener on exit */
	go func() {
		<-d.ctx.Done()
		d.listener.Close()
	}()

	go d.watchdog()

	for d.ctx.Err() == nil {
		conn, err := d.listener.AcceptUnix()
		if err != nil {
			continue
		}
		go d.handleConnection(conn)
	}
}

func (d *daemon) handleConnection(conn *net.UnixConn) {
	d.mu.Lock()
	d.clientCount++
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.clientCount--
		d.mu.Unlock()
		conn.Close()
	}()

	// Handle the connection here
}
