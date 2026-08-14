package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type EventNATS struct {
	subject string
	data    *any
}

type SinkNATS struct {
	ch     chan *EventNATS
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	url    string
}

func newSinkNATS(url string) *SinkNATS {
	ctx, cancel := context.WithCancel(context.Background())

	sink := &SinkNATS{
		ch:     make(chan *EventNATS, 1024),
		url:    url,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go sink.run()

	return sink
}

func (s *SinkNATS) connect() *nats.Conn {
	for s.ctx.Err() == nil {
		conn, err := nats.Connect(s.url, nats.MaxReconnects(-1), nats.ReconnectWait(1*time.Second))
		if err == nil {
			return conn
		}

		slog.Error("failed to connect to NATS", "error", err)
		select {
		case <-s.ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}

	return nil
}

func (s *SinkNATS) run() {
	defer s.cancel()
	defer close(s.done)

	/* Establish the initial conn */
	conn := s.connect()
	if conn == nil {
		return
	}
	defer conn.Close()

	/* We're now connected */
	slog.Info("connected to NATS", "url", s.url)

	/* Publish events as they come */
	for s.ctx.Err() == nil {
		select {
		case event := <-s.ch:
			err := s.publish(conn, event)
			if err != nil {
				slog.Error("failed to publish event", "error", err)
			}
		case <-s.ctx.Done():
		}
	}
}

func (s *SinkNATS) publish(conn *nats.Conn, event *EventNATS) error {
	data, err := json.Marshal(event.data)
	if err != nil {
		return err
	}

	return conn.Publish(event.subject, data)
}

func (s *SinkNATS) Send(subject string, event any) error {
	ev := EventNATS{
		subject: subject,
		data:    &event,
	}

	select {
	case s.ch <- &ev:
	case <-s.ctx.Done():
	default:
	}

	return nil
}

func (s *SinkNATS) Close() error {
	s.cancel()
	<-s.done
	return nil
}
