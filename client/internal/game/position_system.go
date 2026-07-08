package game

import (
	"context"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/ipc"
	"github.com/OoTMM/multiplayer/shared/protocol"
)

type PositionData struct {
	Key uint16
	X   int16
	Y   int16
	Z   int16
}

type PlayerData struct {
	Name [8]byte
	Pos  PositionData
	TTL  int
}

type PositionSystem struct {
	session  *Session
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	pos      PositionData
	otherPos map[[16]byte]PlayerData
}

func createPositionSystem(session *Session) *PositionSystem {
	ctx, cancel := context.WithCancel(context.Background())

	system := &PositionSystem{
		session: session,
		pos: PositionData{
			Key: 0xffff,
		},
		ctx:      ctx,
		cancel:   cancel,
		otherPos: make(map[[16]byte]PlayerData),
		done:     make(chan struct{}),
	}

	go system.run()

	return system
}

func (s *PositionSystem) sendSelfPos() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pos.Key != 0xffff {
		body := protocol.ClientPosition{
			Key: s.pos.Key,
			X:   s.pos.X,
			Y:   s.pos.Y,
			Z:   s.pos.Z,
		}
		pkt := protocol.Packet{Op: protocol.OpPosition, Data: body.Serialize()}
		s.session.TrySendUplink(&pkt)
		s.pos.Key = 0xffff
	}
}

func (s *PositionSystem) tick() {
	s.sendSelfPos()
}

func (s *PositionSystem) run() {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	defer close(s.done)

	for {
		select {
		case <-tick.C:
			s.tick()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *PositionSystem) OnGamePos(msg *ipc.MessageBodyPositionIn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pos.Key = msg.Key
	s.pos.X = msg.X
	s.pos.Y = msg.Y
	s.pos.Z = msg.Z
}

func (s *PositionSystem) OnServerPos(pkt *protocol.ServerPosition) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.otherPos[pkt.ID] = PlayerData{
		Name: pkt.Name,
		Pos: PositionData{
			Key: pkt.Key,
			X:   pkt.X,
			Y:   pkt.Y,
			Z:   pkt.Z,
		},
		TTL: 30,
	}
}

func (s *PositionSystem) Close() {
	s.cancel()
	<-s.done
}
