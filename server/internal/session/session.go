package session

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/server/internal/config"
	"github.com/OoTMM/multiplayer/server/internal/events"
	"github.com/OoTMM/multiplayer/shared/protocol"
	"github.com/OoTMM/multiplayer/shared/wal"
)

type Session struct {
	ID           [16]byte
	Secret       [8]byte
	DataDir      string
	ctx          context.Context
	cancel       context.CancelFunc
	players      map[*Player]struct{}
	playersMutex sync.RWMutex
	wal          *wal.WAL
	sink         events.Sink
}

type Player struct {
	Session     *Session
	ID          [16]byte
	Name        [8]byte
	WorldID     uint8
	WalIndex    uint32
	LastPosTime time.Time
	Conn        net.Conn
	ctx         context.Context
	cancel      context.CancelFunc
	in          chan *protocol.Packet
	out         chan *protocol.Packet
}

func getDataPath(prefix string, sessionID [16]byte) (string, error) {
	dataDir := fmt.Sprintf("%s/sessions/%02x/%030x", prefix, sessionID[0], sessionID[1:])
	err := os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create data directory: %v", err)
	}

	return dataDir, nil
}

func OpenSession(ctx context.Context, conf *config.Config, sessionID [16]byte, sessionSecret [8]byte, sink events.Sink) (*Session, error) {
	ctx, cancel := context.WithCancel(ctx)

	dataPath, err := getDataPath(conf.DataDir, sessionID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get data path: %v", err)
	}

	wal, err := wal.OpenWAL(ctx, fmt.Sprintf("%s/wal.bin", dataPath))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open WAL: %v", err)
	}

	session := &Session{
		ID:      sessionID,
		Secret:  sessionSecret,
		DataDir: dataPath,
		ctx:     ctx,
		cancel:  cancel,
		players: make(map[*Player]struct{}),
		wal:     wal,
		sink:    sink,
	}

	return session, nil
}

func (s *Session) BroadcastExcept(pkt *protocol.Packet, excluded *Player) {
	s.playersMutex.RLock()
	defer s.playersMutex.RUnlock()

	for player := range s.players {
		if player == excluded {
			continue
		}

		select {
		case player.out <- pkt:
		case <-player.ctx.Done():
		}
	}
}

func (s *Session) Broadcast(pkt *protocol.Packet) {
	s.BroadcastExcept(pkt, nil)
}

func (p *Player) handlePositionUpdate(pos *protocol.ClientPosition) {
	/* Server broadcast */
	body := &protocol.ServerPosition{
		ID:   p.ID,
		Name: p.Name,
		Key:  pos.Key,
		X:    pos.X,
		Y:    pos.Y,
		Z:    pos.Z,
	}
	p.Session.BroadcastExcept(&protocol.Packet{
		Op:   protocol.OpPosition,
		Data: body.Serialize(),
	}, p)

	/* Event sink */
	if p.LastPosTime.IsZero() || time.Since(p.LastPosTime) >= 250*time.Millisecond {
		p.LastPosTime = time.Now()

		var eventPosition struct {
			Event      string `json:"event"`
			SessionID  string `json:"sessionId"`
			PlayerID   string `json:"playerId"`
			WorldID    uint8  `json:"worldId"`
			PlayerName string `json:"playerName"`
			Key        uint16 `json:"key"`
			X          int16  `json:"x"`
			Y          int16  `json:"y"`
			Z          int16  `json:"z"`
		}

		eventPosition.Event = "position"
		eventPosition.SessionID = hex.EncodeToString(p.Session.ID[:])
		eventPosition.PlayerID = hex.EncodeToString(p.ID[:])
		eventPosition.WorldID = p.WorldID
		eventPosition.PlayerName = strings.TrimRight(string(p.Name[:]), "\x00")
		eventPosition.Key = pos.Key
		eventPosition.X = pos.X
		eventPosition.Y = pos.Y
		eventPosition.Z = pos.Z

		err := p.Session.sink.Send(fmt.Sprintf("ootmm.events.multi.%s", eventPosition.SessionID[:]), &eventPosition)
		if err != nil {
			slog.Error("failed to send position event", "error", err)
		}
	}
}

func (p *Player) handlePacket(pkt *protocol.Packet) error {
	switch pkt.Op {
	case protocol.OpNOP:
		return nil
	case protocol.OpWal:
		entry, err := wal.Parse(pkt.Data)
		if err != nil {
			return fmt.Errorf("failed to parse WAL entry: %v", err)
		}
		err = p.Session.wal.Append(entry)
		if err != nil {
			return fmt.Errorf("failed to append WAL entry: %v", err)
		}

		/* TODO: Enrich this quite critical log */
		slog.Info("new WAL entry", "sessionId", hex.EncodeToString(p.Session.ID[:]), "playerId", hex.EncodeToString(p.ID[:]))

		/* Ack the WAL entry */
		dedupKey, err := entry.DedupKey()
		if err != nil {
			return fmt.Errorf("failed to compute deduplication key: %v", err)
		}
		p.Send(&protocol.Packet{
			Op:   protocol.OpWalAck,
			Data: dedupKey[:],
		})
	case protocol.OpPosition:
		pos, err := protocol.ParseClientPosition(pkt.Data)
		if err != nil {
			return fmt.Errorf("failed to parse client position: %v", err)
		}
		p.handlePositionUpdate(pos)
	default:
		slog.Warn("Unhandled packet opcode", "op", pkt.Op)
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
				slog.Error("failed to handle packet", "error", err)
				return
			}
		}
	}
}

func (p *Player) handleMsgIn() {
	defer p.cancel()
	for p.ctx.Err() == nil {
		pkt, err := protocol.RecvRawTimeoutDefault(p.Conn)
		if err != nil {
			if p.ctx.Err() == nil {
				if errors.Is(err, io.EOF) {
					slog.Info("client disconnected")
				} else {
					slog.Warn("unexpected disconnect", "error", err)
				}
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
	nopPacket := protocol.Packet{
		Op:   protocol.OpNOP,
		Data: []byte{},
	}
	nopInterval := 3 * time.Second
	nopTimer := time.NewTimer(nopInterval)

	defer nopTimer.Stop()
	defer p.cancel()

	for {
		var pkt *protocol.Packet
		select {
		case pkt = <-p.out:
		case <-nopTimer.C:
			pkt = &nopPacket
		case <-p.ctx.Done():
			return
		}

		if err := protocol.SendRaw(p.Conn, pkt); err != nil {
			if p.ctx.Err() == nil {
				slog.Error("failed to send packet", "error", err)
			}
			return
		}
		nopTimer.Reset(nopInterval)
	}
}

func (p *Player) handleWalStream() {
	defer p.cancel()
	stream := p.Session.wal.Subscribe(p.ctx, p.WalIndex)

	for p.ctx.Err() == nil {
		entry, index, err := stream.Next()
		if err != nil {
			if p.ctx.Err() == nil {
				slog.Error("failed to get next WAL entry", "error", err)
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
			slog.Error("failed to serialize ServerWal", "error", err)
			return
		}

		p.Send(&protocol.Packet{
			Op:   protocol.OpWal,
			Data: bodyData,
		})
	}
}

func (s *Session) Join(PlayerID [16]byte, PlayerName [8]byte, worldID uint8, walIndex uint32, conn net.Conn) {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	player := &Player{
		Session:  s,
		ID:       PlayerID,
		Name:     PlayerName,
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

	defer func() {
		s.playersMutex.Lock()
		delete(s.players, player)
		s.playersMutex.Unlock()
	}()

	/* Reply */
	pkt := &protocol.Packet{
		Op: protocol.OpHello,
		Data: (&protocol.ServerHello{
			Magic:   [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00},
			Version: 0x00010000,
		}).Serialize(),
	}
	err := protocol.SendRaw(conn, pkt)

	if err != nil {
		slog.Error("failed to send hello packet", "error", err)
		return
	}

	wg.Go(player.handleMsgIn)
	wg.Go(player.handleMsgOut)
	wg.Go(player.handlePackets)
	wg.Go(player.handleWalStream)

	<-ctx.Done()
	player.Conn.Close()
	wg.Wait()
}

func (s *Session) Close() {
	s.cancel()
	s.wal.Close()
}

func (p *Player) Send(pkt *protocol.Packet) {
	select {
	case p.out <- pkt:
	case <-p.ctx.Done():
	}
}
