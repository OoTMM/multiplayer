package daemon

import (
	"crypto/rand"
	"sync"
)

type client struct {
	id   [16]byte
	mu   sync.Mutex
	subs map[chan string]struct{}
	done chan struct{}
}

func newClient() *client {
	/* Generate a unique client ID */
	client := &client{
		id:   [16]byte{},
		subs: make(map[chan string]struct{}),
		done: make(chan struct{}),
	}
	_, err := rand.Read(client.id[:])
	if err != nil {
		panic(err)
	}
	return client
}

func (c *client) subscribe() chan string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan string)
	c.subs[ch] = struct{}{}
	return ch
}

func (c *client) unsubscribe(ch chan string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.subs[ch]; !ok {
		return
	}
	close(ch)
	delete(c.subs, ch)
}

func (c *client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for sub := range c.subs {
		close(sub)
		delete(c.subs, sub)
	}
	close(c.done)
}

func (c *client) broadcast(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for sub := range c.subs {
		select {
		case sub <- msg:
		default:
		}
	}
}
