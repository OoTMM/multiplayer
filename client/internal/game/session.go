package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/config"
	"github.com/OoTMM/multiplayer/client/internal/ipc"
	"github.com/OoTMM/multiplayer/shared/protocol"
	"github.com/OoTMM/multiplayer/shared/wal"
)

type Session struct {
	Conf          *config.Config
	Conn          ipc.Conn
	SessionID     [16]byte
	SessionSecret [8]byte
	PlayerID      [16]byte
	PlayerName    [8]byte
	WorldID       uint8
	SeqGame       uint32
	SeqNet        uint32
	msgIn         chan *ipc.Message
	msgOut        chan *ipc.Message
	uplinkIn      chan *protocol.Packet
	uplinkOut     chan *protocol.Packet
	ctx           context.Context
	cancel        context.CancelFunc
	muHello       sync.Mutex
	dataDir       string
	sendQ         *SendQueue
	wal           *wal.WAL

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

		/* Clear the send queue */
		dedupKey, err := body.Entry.DedupKey()
		if err != nil {
			return fmt.Errorf("failed to compute deduplication key: %v", err)
		}
		s.sendQ.Ack(dedupKey)
		fmt.Printf("Received WAL entry from uplink: %+v\n", body.Entry)
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
	dataDir := fmt.Sprintf("%s/sessions/%02x/%030x", prefix, id[0:2], id[2:])
	err := os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", err
	}
	return dataDir, nil
}

func Run(ctx context.Context, conf *config.Config, conn ipc.Conn, hello *ipc.MessageBodyHelloIn) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dataDir, err := makeDataDir(conf.DataDir, hello.SessionID[:])
	if err != nil {
		fmt.Println("failed to create data directory:", err)
		return
	}

	var sendQ *SendQueue
	if hello.Multiplayer {
		sendQ, err = OpenSendQueue(fmt.Sprintf("%s/send_queue.dat", dataDir))
		if err != nil {
			fmt.Println("failed to open send queue:", err)
			return
		}
	}
	defer sendQ.Close()

	wal, err := wal.OpenWAL(fmt.Sprintf("%s/wal.bin", dataDir))
	if err != nil {
		fmt.Println("failed to open WAL:", err)
		return
	}
	defer wal.Close()

	/* Generate random sequence numbers */
	randBytes := make([]byte, 8)
	rand.Read(randBytes)

	seqGame := binary.LittleEndian.Uint32(randBytes[0:4])
	seqNet := binary.LittleEndian.Uint32(randBytes[4:8])

	session := &Session{
		Conf:          conf,
		Conn:          conn,
		SessionID:     hello.SessionID,
		SessionSecret: hello.SessionSecret,
		PlayerID:      hello.PlayerID,
		PlayerName:    hello.PlayerName,
		WorldID:       hello.WorldID,
		SeqGame:       seqGame,
		SeqNet:        seqNet,
		msgIn:         make(chan *ipc.Message, 16),
		msgOut:        make(chan *ipc.Message, 16),
		ctx:           ctx,
		cancel:        cancel,
		uplinkIn:      make(chan *protocol.Packet, 16),
		uplinkOut:     make(chan *protocol.Packet, 16),
		dataDir:       dataDir,
		sendQ:         sendQ,
		wal:           wal,
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
	wg.Go(session.handleUplink)
	wg.Go(session.handleSendQueue)

	/* Wait for cancellation */
	<-session.ctx.Done()
	session.positions.Close()
	session.Conn.Close()
	wg.Wait()

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
	entry.From = p.WorldID

	switch w.Type {
	case wal.WalItem:
		item, err := ipc.ParseWalItemIn(w.Data)
		if err != nil {
			return err
		}
		fmt.Printf("Received WAL item from player %s: %+v\n", p.PlayerName, item)
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
	fmt.Printf("sent ACK: %+v", ackMsg)

	return nil
}

func (s *Session) sendGameWal(index uint32) error {
	for i := uint32(0); i < 16; i++ {
		entry := s.wal.Get(index + i)
		if entry == nil {
			return nil
		}

		/* Send to the game */
		var data []byte
		switch entry.Type {
		case wal.WalItem:
			body := ipc.WalItemOut{
				From:       entry.From,
				To:         entry.Item.To,
				Game:       entry.Item.Game,
				GI:         entry.Item.GI,
				Flags:      entry.Item.Flags,
				Key:        entry.Item.Key,
				PlayerName: entry.PlayerName,
			}
			data = body.Serialize()
		default:
			fmt.Println("warn: unhandled WAL entry type:", entry.Type)
		}

		wrapper := ipc.MessageBodyWalOut{
			Type:  entry.Type,
			Index: index + i,
			Data:  data,
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
		return p.handleWalIn(walMsg)
	case ipc.OpWalQuery:
		index := binary.BigEndian.Uint32(msg.Payload)
		return p.sendGameWal(index)
	case ipc.OpPosition:
		posMsg, err := ipc.ParseMessageBodyPositionIn(msg.Payload)
		if err != nil {
			return err
		}
		p.positions.OnGamePos(posMsg)
		return nil
	default:
		return fmt.Errorf("unhandled message opcode: %d", msg.Op)
	}
}

func (p *Session) Send(msg *ipc.Message) {
	p.msgOut <- msg
}

func (p *Session) TrySend(msg *ipc.Message) {
	select {
	case p.msgOut <- msg:
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
