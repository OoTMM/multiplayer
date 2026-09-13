package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/util"
	"github.com/gofrs/flock"
)

var daemonSocketPath = fmt.Sprintf("%s/internal.sock", util.RunDir())
var daemonLockPath = fmt.Sprintf("%s/internal.lock", util.RunDir())

type daemon struct {
	ctx          context.Context
	cancel       context.CancelFunc
	listener     *net.UnixListener
	httpListener *net.TCPListener
	mu           sync.Mutex
	clients      map[[16]byte]*client
}

func Run() {
	os.MkdirAll(util.RunDir(), 0o700)

	/* Try to acquire the daemon lock */
	lock := flock.New(daemonLockPath)
	ok, err := lock.TryLock()
	if err != nil {
		fmt.Printf("Failed to acquire daemon lock: %v\n", err)
		return
	}
	if !ok {
		fmt.Printf("Failed to acquire daemon lock: already locked\n")
		return
	}

	defer func() {
		lock.Close()
		os.Remove(daemonLockPath)
	}()

	/* Remove a stale socket, if any */
	os.Remove(daemonSocketPath)

	addr, err := net.ResolveUnixAddr("unix", daemonSocketPath)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	rawHttpListener, err := net.Listen("tcp", "127.0.0.1:39278")
	if err != nil {
		panic(err)
	}
	httpListener := rawHttpListener.(*net.TCPListener)
	defer httpListener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	daemon := &daemon{
		ctx:          ctx,
		cancel:       cancel,
		listener:     listener,
		httpListener: httpListener,
		clients:      make(map[[16]byte]*client),
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
			if len(d.clients) == 0 {
				d.cancel()
			}
			d.mu.Unlock()
		}
	}
}

func (d *daemon) run() {
	/* Start the HTTP server */
	go d.httpServer()

	/* Close the listener on exit */
	go func() {
		<-d.ctx.Done()
		d.listener.Close()
		d.httpListener.Close()
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
	client := newClient()

	d.mu.Lock()
	d.clients[client.id] = client
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.clients, client.id)
		d.mu.Unlock()
		conn.Close()
		client.close()
	}()

	for d.ctx.Err() == nil {
		msg, err := recvMsg(conn)
		if err != nil {
			return
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return
		}
		dataStr := string(data)
		fmt.Println(dataStr)
		client.broadcast(dataStr)
	}
}
