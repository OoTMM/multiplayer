package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
)

type sinkConsumer struct {
	conn net.Conn
	ch   chan string
}

type EventSink struct {
	ctx       context.Context
	cancel    context.CancelFunc
	baseDir   string
	path      string
	listener  *net.UnixListener
	consumers map[*sinkConsumer]struct{}
	mu        sync.Mutex
	done      chan struct{}
}

func NewEventSink(ctx context.Context) (*EventSink, error) {
	dir := runDir()
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return nil, err
	}

	sockPath := fmt.Sprintf("%s/%d.sock", dir, os.Getpid())
	addr, err := net.ResolveUnixAddr("unix", sockPath)
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	es := &EventSink{
		ctx:       ctx,
		cancel:    cancel,
		baseDir:   dir,
		path:      sockPath,
		listener:  listener,
		consumers: make(map[*sinkConsumer]struct{}),
		done:      make(chan struct{}),
	}

	go es.run()

	return es, nil
}

func handleConsumer(sink *EventSink, cons *sinkConsumer) {
	defer func() {
		sink.mu.Lock()
		delete(sink.consumers, cons)
		sink.mu.Unlock()
		cons.conn.Close()
	}()

	for msg := range cons.ch {
		_, err := cons.conn.Write([]byte(msg))
		if err != nil {
			slog.Error("Failed to write to consumer", "error", err)
			return
		}
	}
}

func (es *EventSink) run() {
	defer close(es.done)

	for es.ctx.Err() == nil {
		/* Accept new connections */
		conn, err := es.listener.Accept()
		if err != nil {
			if es.ctx.Err() == nil {
				slog.Error("Failed to accept connection", "error", err)
			}
			continue
		}
		unixConn, ok := conn.(*net.UnixConn)
		if !ok {
			slog.Error("Failed to cast connection to UnixConn")
			conn.Close()
			continue
		}
		err = unixConn.CloseRead()
		if err != nil {
			slog.Error("Failed to close read side of connection", "error", err)
			conn.Close()
			continue
		}

		/* Register a consumer */
		cons := &sinkConsumer{conn: conn, ch: make(chan string, 128)}
		cons.ch <- "{\"type\":\"CONNECTED\"}\n"

		es.mu.Lock()
		es.consumers[cons] = struct{}{}
		es.mu.Unlock()

		/* Start a goroutine to handle the consumer */
		go handleConsumer(es, cons)
	}

	/* At this point we are shutting down */
	/* Terminate all consumers */
	es.mu.Lock()
	for cons := range es.consumers {
		close(cons.ch)
		cons.conn.Close()
	}
	es.mu.Unlock()
}

func (es *EventSink) Close() error {
	es.cancel()
	err := es.listener.Close()
	if err != nil {
		return err
	}
	<-es.done
	return nil
}

func (es *EventSink) Emit(t string, data ...any) {
	/* Format the msg as a JSON line */
	d := make(map[string]any)
	d["type"] = t
	for i := 0; i < len(data); i += 2 {
		key, ok := data[i].(string)
		if !ok {
			slog.Error("Emit: key is not a string", "key", data[i])
			continue
		}
		if i+1 >= len(data) {
			slog.Error("Emit: missing value for key", "key", key)
			continue
		}
		d[key] = data[i+1]
	}

	str, err := json.Marshal(d)
	if err != nil {
		slog.Error("Failed to marshal event", "error", err)
		return
	}

	str = append(str, '\n')

	/* Send the msg to all consumers */
	es.mu.Lock()
	for cons := range es.consumers {
		select {
		case cons.ch <- string(str):
		default:
			slog.Warn("Consumer channel full, dropping event")
		}
	}
	es.mu.Unlock()
}
