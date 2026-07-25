package game

import (
	"context"
	"encoding/binary"
	"fmt"
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
	Name    [8]byte
	LocalID uint16
	TTL     int
}

type PositionSystem struct {
	session *Session
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	pos     PositionData

	local       map[[16]byte]*PlayerData
	localIdFree []uint16
	localIdNext uint16
}

func createPositionSystem(session *Session) *PositionSystem {
	ctx, cancel := context.WithCancel(context.Background())

	system := &PositionSystem{
		session: session,
		pos: PositionData{
			Key: 0xffff,
		},
		ctx:         ctx,
		cancel:      cancel,
		local:       make(map[[16]byte]*PlayerData),
		localIdFree: make([]uint16, 0),
		localIdNext: 1,
		done:        make(chan struct{}),
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

func (s *PositionSystem) lowerTTLs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, data := range s.local {
		data.TTL--
		if data.TTL <= 0 {
			delete(s.local, id)
			s.localIdFree = append(s.localIdFree, data.LocalID)
			fmt.Printf("Player %s (ID: %x) timed out and was removed\n", data.Name, id)
		}
	}
}

func (s *PositionSystem) tick() {
	s.sendSelfPos()
	s.lowerTTLs()
}

func (s *PositionSystem) run() {
	tick := time.NewTicker(50 * time.Millisecond)
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

	data, ok := s.local[pkt.ID]
	if !ok {
		fmt.Printf("Received pos from new player: %s (ID: %x)\n", pkt.Name, pkt.ID)
		var localID uint16
		if len(s.localIdFree) > 0 {
			localID = s.localIdFree[len(s.localIdFree)-1]
			s.localIdFree = s.localIdFree[:len(s.localIdFree)-1]
		} else {
			localID = s.localIdNext
			s.localIdNext++
		}
		data = &PlayerData{
			LocalID: localID,
			Name:    pkt.Name,
			TTL:     30,
		}
		s.local[pkt.ID] = data
	} else {
		data.TTL = 30
	}

	body := ipc.MessageBodyPositionOut{
		ID:    data.LocalID,
		Color: binary.BigEndian.Uint16(pkt.ID[0:2]),
		Name:  pkt.Name,
		Key:   pkt.Key,
		X:     pkt.X,
		Y:     pkt.Y,
		Z:     pkt.Z,
	}

	s.session.TrySend(&ipc.Message{
		Op:      ipc.OpPosition,
		Payload: body.Serialize(),
	})
}

func (s *PositionSystem) Close() {
	s.cancel()
	<-s.done
}
