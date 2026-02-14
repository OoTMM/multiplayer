package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/protocol"
)

type ServerLink struct {
	address     string
	sessionInfo *SessionInfo
	wal         *protocol.WAL

	conn net.Conn
	mu   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	packetsIn  chan []byte
	packetsOut chan []byte
}

func drainChannel(ch chan []byte) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func CreateServerLink(address string, sessionInfo *SessionInfo, wal *protocol.WAL) *ServerLink {
	ctx, cancel := context.WithCancel(context.Background())

	sl := &ServerLink{
		address:     address,
		sessionInfo: sessionInfo,
		wal:         wal,
		ctx:         ctx,
		cancel:      cancel,
		packetsIn:   make(chan []byte, 100),
		packetsOut:  make(chan []byte, 100),
	}

	go sl.loop()

	return sl
}

func (sl *ServerLink) connect() (net.Conn, error) {
	/* Establish connection to the server */
	conn, err := net.Dial("tcp", sl.address)
	if err != nil {
		return nil, err
	}

	/* Set TCP_NODELAY to reduce latency */
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	/* Send a HELLO packet to the server */
	hello := protocol.ClientHello{
		SessionID:     sl.sessionInfo.SessionID,
		SessionSecret: sl.sessionInfo.SessionSecret,
		PlayerID:      sl.sessionInfo.PlayerID,
		PlayerName:    sl.sessionInfo.NameData,
		WorldID:       sl.sessionInfo.WorldID,
		WalIndex:      sl.wal.Count(),
	}
	err = protocol.NetPacketSend(conn, protocol.SerializeClientHello(&hello))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send hello packet: %v", err)
	}

	/* TODO: Wait for reply */

	return conn, nil
}

func (sl *ServerLink) readLoop(conn net.Conn, ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	defer conn.Close()

	for {
		data, err := protocol.NetPacketRecv(conn)
		if err != nil {
			select {
			case <-ctx.Done():
			default:
				fmt.Printf("Failed to receive packet: %v\n", err)
			}
			return
		}

		select {
		case sl.packetsIn <- data:
		case <-ctx.Done():
			return
		}
	}
}

func (sl *ServerLink) writeLoop(conn net.Conn, ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	defer conn.Close()

	for {
		select {
		case data := <-sl.packetsOut:
			err := protocol.NetPacketSend(conn, data)
			if err != nil {
				select {
				case <-ctx.Done():
				default:
					fmt.Printf("Failed to send packet: %v\n", err)
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (sl *ServerLink) heartbeatLoop(conn net.Conn, ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			sl.packetsIn <- []byte{}
		}
	}
}

func (sl *ServerLink) loop() {
	for {
		var conn net.Conn

		/* Connection loop */
		for {
			var err error

			/* Check if we should stop */
			select {
			case <-sl.ctx.Done():
				return
			default:
			}

			conn, err = sl.connect()
			if err != nil {
				fmt.Printf("Failed to connect to server: %v\n", err)
				time.Sleep(5 * time.Second)
				continue
			} else {
				break
			}
		}

		/* Drain channels */
		drainChannel(sl.packetsIn)
		drainChannel(sl.packetsOut)

		/* Store the connection */
		sl.mu.Lock()
		sl.conn = conn
		sl.mu.Unlock()

		/* Create a local context for the current connection */
		connCtx, connCancel := context.WithCancel(sl.ctx)

		/* Start read/write loops */
		sl.wg.Go(func() { sl.readLoop(conn, connCtx, connCancel) })
		sl.wg.Go(func() { sl.writeLoop(conn, connCtx, connCancel) })
		sl.wg.Go(func() { sl.heartbeatLoop(conn, connCtx, connCancel) })

		/* Wait for the connection to be closed */
		<-connCtx.Done()

		/* Unset the connection */
		sl.mu.Lock()
		sl.conn = nil
		sl.mu.Unlock()

		/* Clear the old connection */
		conn.Close()

		/* Wait for the group */
		sl.wg.Wait()
	}
}

func (sl *ServerLink) Close() {
	sl.cancel()
	sl.wg.Wait()
}
