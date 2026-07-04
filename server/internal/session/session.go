package session

import (
	"context"
	"net"
	"sync"
)

type Session struct {
	ID           [16]byte
	Secret       [8]byte
	ctx          context.Context
	players      map[*Player]struct{}
	playersMutex sync.RWMutex
}

func OpenSession(ctx context.Context, sessionID [16]byte, sessionSecret [8]byte) *Session {
	session := &Session{
		ID:     sessionID,
		Secret: sessionSecret,
		ctx:    ctx,
	}

	return session
}

func (s *Session) Join(PlayerID [16]byte, worldID uint8, walIndex uint32, conn net.Conn) *Player {
	player := &Player{
		Session:  s,
		ID:       PlayerID,
		WorldID:  worldID,
		WalIndex: walIndex,
		Conn:     conn,
	}

	s.playersMutex.Lock()
	s.players[player] = struct{}{}
	s.playersMutex.Unlock()

	return player
}
