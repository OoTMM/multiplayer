package session

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/OoTMM/multiplayer/shared/protocol"
)

type Session struct {
	ID           [16]byte
	Secret       [8]byte
	ctx          context.Context
	players      map[*Player]struct{}
	playersMutex sync.RWMutex
}

type Player struct {
	Session  *Session
	ID       [16]byte
	WorldID  uint8
	WalIndex uint32
	Conn     net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	in       chan *protocol.Packet
	out      chan *protocol.Packet
}

func OpenSession(ctx context.Context, sessionID [16]byte, sessionSecret [8]byte) *Session {
	session := &Session{
		ID:      sessionID,
		Secret:  sessionSecret,
		ctx:     ctx,
		players: make(map[*Player]struct{}),
	}

	return session
}

func (p *Player) handlePacket(pkt *protocol.Packet) error {
	fmt.Printf("Received packet from player: Op=%d, Data=%x\n", pkt.Op, pkt.Data)
	return nil
}

func (p *Player) handlePackets() {
	defer p.cancel()
	for p.ctx.Err() == nil {
		select {
		case <-p.ctx.Done():
			return
		case pkt := <-p.in:
			err := p.handlePacket(pkt)
			if err != nil {
				fmt.Println("Failed to handle packet:", err)
				return
			}
		}
	}
}

func (p *Player) handleMsgIn() {
	defer p.cancel()
	for p.ctx.Err() == nil {
		pkt, err := protocol.RecvRaw(p.Conn)
		if err != nil {
			if p.ctx.Err() == nil {
				fmt.Println("Failed to receive packet:", err)
			}
			return
		}

		select {
		case p.in <- pkt:
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *Player) handleMsgOut() {
	defer p.cancel()

	for p.ctx.Err() == nil {
		select {
		case pkt := <-p.out:
			err := protocol.SendRaw(p.Conn, pkt)
			if err != nil {
				if p.ctx.Err() == nil {
					fmt.Println("Failed to send packet:", err)
				}
				return
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (s *Session) Join(PlayerID [16]byte, worldID uint8, walIndex uint32, conn net.Conn) {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(s.ctx)

	player := &Player{
		Session:  s,
		ID:       PlayerID,
		WorldID:  worldID,
		WalIndex: walIndex,
		Conn:     conn,
		ctx:      ctx,
		cancel:   cancel,
		in:       make(chan *protocol.Packet, 16),
		out:      make(chan *protocol.Packet, 16),
	}

	s.playersMutex.Lock()
	s.players[player] = struct{}{}
	s.playersMutex.Unlock()

	/* Reply */
	pkt := &protocol.Packet{
		Op: protocol.OpHello,
		Data: (&protocol.ServerHello{
			Magic:   [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00},
			Version: 0x00010000,
		}).Serialize(),
	}
	err := protocol.SendRaw(conn, pkt)
	fmt.Printf("Sent hello packet to player %s: %+v\n", player.ID, pkt)

	if err == nil {
		wg.Go(player.handleMsgIn)
		wg.Go(player.handleMsgOut)
		wg.Go(player.handlePackets)
		<-ctx.Done()
	} else {
		fmt.Println("Failed to send hello packet:", err)
		cancel()
	}

	player.Conn.Close()

	s.playersMutex.Lock()
	delete(s.players, player)
	s.playersMutex.Unlock()
}
