package session

import "net"

type Player struct {
	Session  *Session
	ID       [16]byte
	WorldID  uint8
	WalIndex uint32
	Conn     net.Conn
}
