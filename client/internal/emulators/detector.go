package emulators

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Detector struct {
	ctx         context.Context
	connectors  chan Connector
	keys        map[string]Connector
	keysReverse map[Connector]string
	wg          sync.WaitGroup
}

func NewDetector(ctx context.Context) *Detector {
	det := &Detector{
		ctx:         ctx,
		connectors:  make(chan Connector, 8),
		keys:        make(map[string]Connector),
		keysReverse: make(map[Connector]string),
	}

	det.wg.Go(det.pollGDB)

	return det
}

func (d *Detector) Close() {
	d.wg.Wait()
	for conn := range d.keysReverse {
		conn.Close()
	}
}

func (d *Detector) Connectors() <-chan Connector {
	return d.connectors
}

func (d *Detector) pollGDB() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			key := "gdb:9123"
			if _, exists := d.keys[key]; exists {
				continue
			}
			conn := ConnectGDB(d.ctx)
			if conn != nil {
				fmt.Println("Detected GDB-compatible emulator")
				d.keys[key] = conn
				d.keysReverse[conn] = key
				d.connectors <- conn
			}
		}
	}
}
