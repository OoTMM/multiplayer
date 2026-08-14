package events

import "github.com/OoTMM/multiplayer/server/internal/config"

type Sink interface {
	Send(subject string, event any) error
	Close() error
}

func NewSink(config *config.Config) Sink {
	if config.NatsURL != "" {
		return newSinkNATS(config.NatsURL)
	} else {
		return newSinkNull()
	}
}
