package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/config"
	"github.com/OoTMM/multiplayer/client/internal/events"
	"github.com/OoTMM/multiplayer/client/internal/ipc"
	"github.com/OoTMM/multiplayer/shared/protocol"
	"github.com/OoTMM/multiplayer/shared/wal"
)

type Session struct {
	Conf       *config.Config
	Events     *events.EventSink
	Info       *Info
	Conn       ipc.Conn
	PlayerID   [16]byte
	PlayerName [8]byte
	SeqGame    uint32
	SeqNet     uint32
	msgIn      chan *ipc.Message
	msgOut     chan *ipc.Message
	uplinkIn   chan *protocol.Packet
	uplinkOut  chan *protocol.Packet
	ctx        context.Context
	cancel     context.CancelFunc
	muHello    sync.Mutex
	dataDir    string
	sendQ      *SendQueue
	wal        *wal.WAL

	positions *PositionSystem
}

func (s *Session) handleUplinkPacket(pkt *protocol.Packet) error {
	switch pkt.Op {
	case protocol.OpNOP:
		return nil
	case protocol.OpWal:
		body, err := protocol.ParseServerWal(pkt.Data)
		if err != nil {
			return fmt.Errorf("failed to parse server WAL packet: %v", err)
		}

		/* Append to the WAL */
		err = s.wal.Append(body.Entry)
		if err != nil {
			return fmt.Errorf("failed to append WAL entry: %v", err)
		}

		/* Clear the send queue */
		dedupKey, err := body.Entry.DedupKey()
		if err != nil {
			return fmt.Errorf("failed to compute deduplication key: %v", err)
		}
		s.sendQ.Ack(dedupKey)

		switch body.Entry.Type {
		case wal.WalItem:
			fmt.Printf("Server: Item (PlayerID=%032x, PlayerName=%s, From=%d, To=%d, Game=%d, GI=%d, Flags=%04x, Key=%08x)\n", body.Entry.PlayerID, body.Entry.PlayerName, body.Entry.From, body.Entry.Item.To, body.Entry.Item.Game, body.Entry.Item.GI, body.Entry.Item.Flags, body.Entry.Item.Key)
		case wal.WalEvent:
			fmt.Printf("Server: Event (PlayerID=%032x, PlayerName=%s, From=%d, EventID=%08x)\n", body.Entry.PlayerID, body.Entry.PlayerName, body.Entry.From, body.Entry.Event.ID)
		default:
			fmt.Printf("Server: Unknown: %+v\n", body.Entry)
		}
	case protocol.OpWalAck:
		if len(pkt.Data) != 16 {
			return fmt.Errorf("invalid WAL ACK packet length: %d", len(pkt.Data))
		}
		var dedupKey [16]byte
		copy(dedupKey[:], pkt.Data)
		s.sendQ.Ack(dedupKey)
	case protocol.OpPosition:
		posMsg, err := protocol.ParseServerPosition(pkt.Data)
		if err != nil {
			return fmt.Errorf("failed to parse server position packet: %v", err)
		}
		s.positions.OnServerPos(posMsg)
	default:
		fmt.Printf("warn: unhandled uplink packet: Op=%d, Data=%x\n", pkt.Op, pkt.Data)
	}

	return nil
}

func makeDataDir(prefix string, id []byte) (string, error) {
	dataDir := fmt.Sprintf("%s/sessions/%02x/%030x", prefix, id[0], id[1:])
	err := os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", err
	}
	return dataDir, nil
}

func Run(ctx context.Context, conf *config.Config, info *Info, conn ipc.Conn, hello *ipc.MessageBodyHelloIn, events *events.EventSink) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if hello.SessionID != info.SessionID || hello.SessionSecret != info.SessionSecret {
		fmt.Println("Session mismatch between game and patchfile")
		return
	}

	if hello.WorldID != info.WorldID {
		fmt.Println("World ID mismatch between game and patchfile")
		return
	}

	dataDir, err := makeDataDir(conf.DataDir, hello.SessionID[:])
	if err != nil {
		fmt.Println("failed to create data directory:", err)
		return
	}

	var sendQ *SendQueue
	if info.Mode != InfoModeSingle {
		sendQ, err = OpenSendQueue(fmt.Sprintf("%s/send_queue.dat", dataDir))
		if err != nil {
			fmt.Println("failed to open send queue:", err)
			return
		}
		defer sendQ.Close()
	}

	wal, err := wal.OpenWAL(ctx, fmt.Sprintf("%s/wal.bin", dataDir))
	if err != nil {
		fmt.Println("failed to open WAL:", err)
		return
	}
	defer wal.Close()

	/* Display the connection notification */
	fmt.Printf("Game connected (PlayerID=%032x, PlayerName=%s)\n", hello.PlayerID, hello.PlayerName)

	/* Generate random sequence numbers */
	randBytes := make([]byte, 8)
	rand.Read(randBytes)

	seqGame := binary.LittleEndian.Uint32(randBytes[0:4])
	seqNet := binary.LittleEndian.Uint32(randBytes[4:8])

	session := &Session{
		Conf:       conf,
		Events:     events,
		Info:       info,
		Conn:       conn,
		PlayerID:   hello.PlayerID,
		PlayerName: hello.PlayerName,
		SeqGame:    seqGame,
		SeqNet:     seqNet,
		msgIn:      make(chan *ipc.Message, 16),
		msgOut:     make(chan *ipc.Message, 16),
		ctx:        ctx,
		cancel:     cancel,
		uplinkIn:   make(chan *protocol.Packet, 16),
		uplinkOut:  make(chan *protocol.Packet, 16),
		dataDir:    dataDir,
		sendQ:      sendQ,
		wal:        wal,
	}

	/* Queue the HELLO OUT message */
	helloOutBody := ipc.MessageBodyHelloOut{
		Magic:   [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00},
		SeqGame: seqGame,
		SeqNet:  seqNet,
	}
	helloOut := ipc.Message{
		Seq:     0,
		Op:      ipc.OpHello,
		Payload: helloOutBody.Serialize(),
	}
	session.Conn.Write(helloOut.Serialize())

	/* Create systems */
	session.positions = createPositionSystem(session)
	defer session.positions.Close()

	/* Start helper I/O goroutines */
	wg.Go(session.handleMsgIn)
	wg.Go(session.handleMsgOut)
	wg.Go(session.handleMsgLoop)

	if session.Info.Mode != InfoModeSingle {
		wg.Go(session.handleUplink)
		wg.Go(session.handleSendQueue)
	}

	/* Log a game start event */
	events.Emit("GAME_START",
		"sessionId", hex.EncodeToString(session.Info.SessionID[:]),
		"playerId", hex.EncodeToString(session.PlayerID[:]),
		"playerName", strings.TrimRight(string(session.PlayerName[:]), "\x00"),
		"worldId", fmt.Sprintf("%d", session.Info.WorldID),
	)

	/* Wait for cancellation */
	<-session.ctx.Done()
	session.positions.Close()
	session.Conn.Close()
	wg.Wait()

	/* Log a game end event */
	events.Emit("GAME_END")

	fmt.Println("Session closed")
}

func (s *Session) handleMsgIn() {
	defer s.cancel()

	for {
		data, err := s.Conn.Read()
		if err != nil {
			if s.ctx.Err() == nil {
				fmt.Println("failed to read from IPC:", err)
			}
			return
		}
		msg, err := ipc.ParseMessage(data)
		if err != nil {
			fmt.Println("failed to parse message")
			return
		}

		select {
		case s.msgIn <- msg:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) handleMsgOut() {
	defer s.cancel()

	for {
		select {
		case msg := <-s.msgOut:
			msg.Seq = s.SeqNet
			s.SeqNet++
			err := s.Conn.Write(msg.Serialize())
			if err != nil {
				fmt.Println("failed to write to IPC")
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) handleSendQueue() {
	defer s.cancel()
	for s.ctx.Err() == nil {
		pending := s.sendQ.Pending()
		for _, data := range pending {
			_, err := wal.Parse(data)
			if err != nil {
				fmt.Println("failed to parse WAL entry from send queue:", err)
				return
			}
			pkt := protocol.Packet{Op: protocol.OpWal, Data: data}
			s.SendUplink(&pkt)
		}
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (p *Session) newWalEntry(entry *wal.WalEntry) error {
	/* Append to the send queue */
	data, err := entry.Serialize()
	if err != nil {
		return err
	}

	dedupKey, err := entry.DedupKey()
	if err != nil {
		return err
	}

	p.sendQ.Add(dedupKey, data)

	/* Send the WAL entry to the uplink (optimization: don't wait for the uplink to read from the send queue) */
	pkt := protocol.Packet{Op: protocol.OpWal, Data: data}
	p.TrySendUplink(&pkt)

	return nil
}

func (p *Session) handleWalIn(w *ipc.MessageBodyWalIn) error {
	var entry wal.WalEntry

	copy(entry.PlayerID[:], p.PlayerID[:])
	copy(entry.PlayerName[:], p.PlayerName[:])
	entry.From = p.Info.WorldID

	switch w.Type {
	case wal.WalItem:
		item, err := ipc.ParseWalItem(w.Data)
		if err != nil {
			return err
		}

		/* Log */
		fmt.Printf("Game: Item (To=%d, Game=%d, GI=%d, Flags=%04x, Key=%08x)\n", item.To, item.Game, item.GI, item.Flags, item.Key)

		/* Send the event */
		p.Events.Emit("GAME_ITEM", "gi", fmt.Sprintf("%d", item.GI))

		entry.Type = wal.WalItem
		entry.Item.To = item.To
		entry.Item.Game = item.Game
		entry.Item.GI = item.GI
		entry.Item.Flags = item.Flags
		entry.Item.Key = item.Key

		/* 0x0001 is OVF_RENEW, this should cause a nonce to be used */
		if item.Flags&0x0001 != 0 {
			var nonce [4]byte
			rand.Read(nonce[:])
			entry.Item.Nonce = binary.LittleEndian.Uint32(nonce[:])
		} else {
			entry.Item.Nonce = 0
		}
	case wal.WalEvent:
		event, err := ipc.ParseWalEvent(w.Data)
		if err != nil {
			return err
		}
		fmt.Printf("Game: Event (ID=%08x)\n", event.EventID)
		entry.Type = wal.WalEvent
		entry.Event.ID = event.EventID
	default:
		return fmt.Errorf("unhandled WAL type: %d", w.Type)
	}

	err := p.newWalEntry(&entry)
	if err != nil {
		return fmt.Errorf("failed to create new WAL entry: %v", err)
	}

	/* ACK the WAL entry */
	var payload [4]byte
	binary.BigEndian.PutUint32(payload[:], w.Token)
	ackMsg := ipc.Message{
		Op:      ipc.OpWalAck,
		Payload: payload[:],
	}
	p.Send(&ackMsg)

	return nil
}

func (s *Session) sendGameWal(index uint32) error {
	for i := uint32(0); i < 16; i++ {
		entry := s.wal.Get(index + i)
		if entry == nil {
			return nil
		}

		/* Send the player name ONLY if the entry is from a different player */
		var playerName [8]byte
		if entry.PlayerID != s.PlayerID {
			copy(playerName[:], entry.PlayerName[:])
		}

		/* Send to the game */
		var data []byte
		switch entry.Type {
		case wal.WalItem:
			body := ipc.WalItem{
				To:    entry.Item.To,
				Game:  entry.Item.Game,
				GI:    entry.Item.GI,
				Flags: entry.Item.Flags,
				Key:   entry.Item.Key,
			}
			data = body.Serialize()
		case wal.WalEvent:
			body := ipc.WalEvent{
				EventID: entry.Event.ID,
			}
			data = body.Serialize()
		default:
			fmt.Println("warn: unhandled WAL entry type:", entry.Type)
		}

		wrapper := ipc.MessageBodyWalOut{
			Index:      index + i,
			Type:       entry.Type,
			From:       entry.From,
			PlayerName: playerName,
			Data:       data,
		}

		msg := ipc.Message{
			Op:      ipc.OpWal,
			Payload: wrapper.Serialize(),
		}

		s.Send(&msg)
	}
	return nil
}

func (p *Session) handleMsg(msg *ipc.Message) error {
	var expectedSeq uint32
	if msg.Op == ipc.OpHello {
		expectedSeq = 0
	} else {
		expectedSeq = p.SeqGame
		p.SeqGame++
	}

	if msg.Seq != expectedSeq {
		return fmt.Errorf("unexpected sequence number: got %d, expected %d", msg.Seq, expectedSeq)
	}

	switch msg.Op {
	case ipc.OpHello:
		return fmt.Errorf("unexpected HELLO message from game")
	case ipc.OpWal:
		walMsg, err := ipc.ParseMessageBodyWalIn(msg.Payload)
		if err != nil {
			return err
		}

		if p.Info.Mode != InfoModeSingle {
			err = p.handleWalIn(walMsg)
		}
		return err
	case ipc.OpWalQuery:
		if len(msg.Payload) < 4 {
			return fmt.Errorf("WAL_QUERY message too short")
		}
		index := binary.BigEndian.Uint32(msg.Payload)
		return p.sendGameWal(index)
	case ipc.OpPosition:
		posMsg, err := ipc.ParseMessageBodyPositionIn(msg.Payload)
		if err != nil {
			return err
		}

		if p.Info.Mode != InfoModeSingle {
			p.positions.OnGamePos(posMsg)
		}
		return nil
	default:
		return fmt.Errorf("unhandled message opcode: %d", msg.Op)
	}
}

func (p *Session) Send(msg *ipc.Message) {
	select {
	case p.msgOut <- msg:
	case <-p.ctx.Done():
	}
}

func (p *Session) TrySend(msg *ipc.Message) {
	select {
	case p.msgOut <- msg:
	case <-p.ctx.Done():
	default:
	}
}

func (p *Session) handleMsgLoop() {
	defer p.cancel()
	for {
		select {
		case msg := <-p.msgIn:
			err := p.handleMsg(msg)
			if err != nil {
				fmt.Println("failed to handle message:", err)
				return
			}
		case <-p.ctx.Done():
			return
		}
	}
}
