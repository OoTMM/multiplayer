package game

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/shared/protocol"
)

type Uplink struct {
	session *Session
	conn    net.Conn
	ctx     context.Context
	cancel  context.CancelFunc
}

func (s *Session) handleUplink() {
	for s.ctx.Err() == nil {
		s.handleUplinkConn()
		if s.ctx.Err() == nil {
			fmt.Println("Uplink connection lost, retrying in 5 seconds...")
			select {
			case <-time.After(5 * time.Second):
			case <-s.ctx.Done():
			}
		}
	}
}

func (s *Session) handleUplinkConn() {
	/* Create a new context for this connection */
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	/* Connect */
	addr := net.JoinHostPort(s.Conf.UpstreamServer, strconv.Itoa(int(s.Conf.UpstreamPort)))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		if s.ctx.Err() == nil {
			fmt.Println("Failed to connect to uplink:", err)
		}
		return
	}
	defer conn.Close()

	/* Drain the channels */
	for {
		select {
		case <-s.uplinkIn:
		case <-s.uplinkOut:
		default:
			goto drain_done
		}
	}
drain_done:

	/* Create the actual uplink */
	uplink := &Uplink{
		session: s,
		conn:    conn,
		ctx:     ctx,
		cancel:  cancel,
	}

	/* Handshake */
	err = uplink.handshake()
	if err != nil {
		if s.ctx.Err() == nil {
			fmt.Println("Failed to handshake with uplink:", err)
		}
		return
	}

	/* Start helper goroutines */
	wg := sync.WaitGroup{}
	wg.Go(uplink.pumpIn)
	wg.Go(uplink.pumpOut)
	wg.Go(uplink.handleNops)
	wg.Go(uplink.handlePackets)

	/* Sync */
	<-ctx.Done()
	cancel()
	conn.Close()
	wg.Wait()
}

func (u *Uplink) handshake() error {
	/* Send hello packet */
	/* TODO: Capture session data properly */
	helloData := &protocol.ClientHello{
		Magic:         [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00},
		Version:       0x00010000,
		SessionID:     u.session.Info.SessionID,
		SessionSecret: u.session.Info.SessionSecret,
		WorldID:       u.session.Info.WorldID,
		PlayerID:      u.session.PlayerID,
		PlayerName:    u.session.PlayerName,
		WalIndex:      u.session.wal.Count(),
	}

	helloPkt := &protocol.Packet{
		Op:   protocol.OpHello,
		Data: helloData.Serialize(),
	}
	err := protocol.SendRaw(u.conn, helloPkt)
	if err != nil {
		return err
	}

	/* Wait for server reply */
	pkt, err := protocol.RecvRawTimeoutDefault(u.conn)
	if err != nil {
		return err
	}

	if pkt.Op != protocol.OpHello {
		return fmt.Errorf("unexpected operation code: %d", pkt.Op)
	}
	if len(pkt.Data) < 12 {
		return fmt.Errorf("server hello too short")
	}
	if string(pkt.Data[0:8]) != "OoTMM\x7f\x01\x00" {
		return fmt.Errorf("invalid magic in server hello")
	}
	if binary.LittleEndian.Uint32(pkt.Data[8:12]) != 0x00010000 {
		return fmt.Errorf("invalid version in server hello")
	}
	fmt.Printf("Connected to server %s:%d\n", u.session.Conf.UpstreamServer, u.session.Conf.UpstreamPort)
	return nil
}

func (u *Uplink) pumpIn() {
	defer u.cancel()
	for u.ctx.Err() == nil {
		pkt, err := protocol.RecvRawTimeoutDefault(u.conn)
		if err != nil {
			if u.ctx.Err() == nil {
				fmt.Println("Failed to receive packet from uplink:", err)
			}
			return
		}

		select {
		case u.session.uplinkIn <- pkt:
		case <-u.ctx.Done():
			return
		}
	}
}

func (u *Uplink) pumpOut() {
	defer u.cancel()
	for u.ctx.Err() == nil {
		select {
		case pkt := <-u.session.uplinkOut:
			err := protocol.SendRaw(u.conn, pkt)
			if err != nil {
				if u.ctx.Err() == nil {
					fmt.Println("Failed to send packet to uplink:", err)
				}
				return
			}
		case <-u.ctx.Done():
			return
		}
	}
}

func (u *Uplink) handleNops() {
	pkt := &protocol.Packet{
		Op:   protocol.OpNOP,
		Data: []byte{},
	}

	defer u.cancel()
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	for u.ctx.Err() == nil {
		select {
		case <-tick.C:
			u.session.TrySendUplink(pkt)
		case <-u.ctx.Done():
			return
		}
	}
}

func (u *Uplink) handlePackets() {
	defer u.cancel()
	for u.ctx.Err() == nil {
		var pkt *protocol.Packet
		select {
		case pkt = <-u.session.uplinkIn:
		case <-u.ctx.Done():
			return
		}

		err := u.session.handleUplinkPacket(pkt)
		if err != nil {
			fmt.Println("failed to handle uplink packet:", err)
			return
		}
	}
}

func (s *Session) SendUplink(pkt *protocol.Packet) {
	select {
	case s.uplinkOut <- pkt:
	case <-s.ctx.Done():
	}
}

func (s *Session) TrySendUplink(pkt *protocol.Packet) {
	if s.ctx.Err() != nil {
		return
	}

	select {
	case s.uplinkOut <- pkt:
	default:
	}
}
