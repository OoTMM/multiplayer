package game

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/shared/protocol"
)

type UplinkHello struct {
	SessionID     [16]byte
	SessionSecret [8]byte
	PlayerID      [16]byte
	PlayerName    [8]byte
	WorldID       uint8
	WalIndex      uint32
}

type Uplink struct {
	ctx        context.Context
	addr       string
	conn       net.Conn
	connCtx    context.Context
	connCancel context.CancelFunc
	in         chan *protocol.Packet
	out        chan *protocol.Packet
	err        error
}

func CreateUplink(ctx context.Context, addr string) *Uplink {
	return &Uplink{
		ctx:  ctx,
		addr: addr,
		in:   make(chan *protocol.Packet, 16),
		out:  make(chan *protocol.Packet, 16),
	}
}

func (u *Uplink) pumpIn() {
	defer u.connCancel()
	for u.connCtx.Err() == nil {
		pkt, err := protocol.RecvRaw(u.conn)
		if err != nil {
			if u.connCtx.Err() == nil {
				u.err = err
			}
			return
		}
		select {
		case u.in <- pkt:
		case <-u.connCtx.Done():
			return
		}
	}
}

func (u *Uplink) pumpOut() {
	defer u.connCancel()
	for {
		select {
		case pkt := <-u.out:
			err := protocol.SendRaw(u.conn, pkt)
			if err != nil {
				if u.connCtx.Err() == nil {
					u.err = err
				}
				return
			}
		case <-u.connCtx.Done():
			return
		}
	}
}

func (u *Uplink) Run(hello *UplinkHello) error {
	wg := sync.WaitGroup{}
	connCtx, connCancel := context.WithCancel(u.ctx)
	defer connCancel()

	/* Drain any existing packets */
	for {
		select {
		case <-u.out:
		case <-u.in:
		default:
			goto drain_done
		}
	}
drain_done:

	/* Create the socket */
	conn, err := net.DialTimeout("tcp", u.addr, 10*time.Second)
	if err != nil {
		return err
	}

	u.err = nil
	u.conn = conn
	u.connCtx = connCtx
	u.connCancel = connCancel

	/* Send hello packet */
	helloData := &protocol.ClientHello{
		Magic:         [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00},
		Version:       0x00010000,
		SessionID:     hello.SessionID,
		SessionSecret: hello.SessionSecret,
		PlayerID:      hello.PlayerID,
		PlayerName:    hello.PlayerName,
		WorldID:       hello.WorldID,
		WalIndex:      hello.WalIndex,
	}

	helloPkt := &protocol.Packet{
		Op:   protocol.OpHello,
		Data: helloData.Serialize(),
	}
	err = protocol.SendRaw(u.conn, helloPkt)
	if err != nil {
		return err
	}

	fmt.Println("Sent hello packet to server")

	/* Start I/O */
	wg.Go(u.pumpIn)
	wg.Go(u.pumpOut)

	/* Wait for context cancellation */
	<-connCtx.Done()
	conn.Close()
	connCancel()
	wg.Wait()

	return u.err
}

func (u *Uplink) Send(pkt *protocol.Packet) {
	select {
	case u.out <- pkt:
	case <-u.connCtx.Done():
	}
}

func (u *Uplink) Recv() (*protocol.Packet, error) {
	select {
	case pkt := <-u.in:
		return pkt, nil
	case <-u.connCtx.Done():
		return nil, u.err
	}
}
