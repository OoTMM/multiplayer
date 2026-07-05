package session

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/OoTMM/multiplayer/shared/protocol"
	"github.com/OoTMM/multiplayer/shared/wal"
)

type Session struct {
	ID           [16]byte
	Secret       [8]byte
	DataDir      string
	ctx          context.Context
	players      map[*Player]struct{}
	playersMutex sync.RWMutex
	wal          *wal.WAL
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

func getDataPath(sessionID [16]byte) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}

	dataDir := fmt.Sprintf("%s/data/server/sessions/%02x/%030x", cwd, sessionID[0:2], sessionID[2:])
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create data directory: %v", err)
	}

	return dataDir, nil
}

func OpenSession(ctx context.Context, sessionID [16]byte, sessionSecret [8]byte) *Session {
	dataPath, err := getDataPath(sessionID)
	if err != nil {
		fmt.Println("Failed to get data path:", err)
		return nil
	}

	wal, err := wal.OpenWAL(fmt.Sprintf("%s/wal.bin", dataPath))
	if err != nil {
		fmt.Println("Failed to open WAL:", err)
		return nil
	}

	session := &Session{
		ID:      sessionID,
		Secret:  sessionSecret,
		DataDir: dataPath,
		ctx:     ctx,
		players: make(map[*Player]struct{}),
		wal:     wal,
	}

	return session
}

func (p *Player) handlePacket(pkt *protocol.Packet) error {
	fmt.Printf("Received packet from player: Op=%d, Data=%x\n", pkt.Op, pkt.Data)
	switch pkt.Op {
	case protocol.OpWal:
		entry, err := wal.Parse(pkt.Data)
		if err != nil {
			return fmt.Errorf("failed to parse WAL entry: %v", err)
		}
		err = p.Session.wal.Append(entry)
		if err != nil {
			return fmt.Errorf("failed to append WAL entry: %v", err)
		}
		fmt.Printf("Appended WAL entry: %+v\n", entry)

		/* Ack the WAL entry */
		dedupKey, err := entry.DedupKey()
		if err != nil {
			return fmt.Errorf("failed to compute deduplication key: %v", err)
		}
		p.Send(&protocol.Packet{
			Op:   protocol.OpWalAck,
			Data: dedupKey[:],
		})
	}
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

func (p *Player) handleWalStream() {
	defer p.cancel()
	stream := p.Session.wal.Subscribe(p.WalIndex)

	for p.ctx.Err() == nil {
		entry, index, err := stream.Next()
		if err != nil {
			if p.ctx.Err() == nil {
				fmt.Println("Failed to get next WAL entry:", err)
			}
			return
		}

		/* Update index */
		if index < p.WalIndex {
			continue
		}
		p.WalIndex = index + 1

		/* Send WAL entry to player */
		body := &protocol.ServerWal{
			Index: index,
			Entry: entry,
		}
		bodyData, err := body.Serialize()
		if err != nil {
			fmt.Println("Failed to serialize ServerWal:", err)
			return
		}

		p.Send(&protocol.Packet{
			Op:   protocol.OpWal,
			Data: bodyData,
		})

		fmt.Printf("Sent WAL entry to player %s: Index=%d, Entry=%+v\n", p.ID, index, entry)
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
		wg.Go(player.handleWalStream)
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

func (p *Player) Send(pkt *protocol.Packet) {
	select {
	case p.out <- pkt:
	case <-p.ctx.Done():
	}
}
