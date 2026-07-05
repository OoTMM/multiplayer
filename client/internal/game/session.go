package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/OoTMM/multiplayer/client/internal/ipc"
	"github.com/OoTMM/multiplayer/shared/protocol"
	"github.com/OoTMM/multiplayer/shared/wal"
)

type Session struct {
	Conn          ipc.Conn
	SessionID     [16]byte
	SessionSecret [8]byte
	PlayerID      [16]byte
	PlayerName    [8]byte
	WorldID       uint8
	WalIndex      uint32
	SeqGame       uint32
	SeqNet        uint32
	msgIn         chan *ipc.Message
	msgOut        chan []byte
	uplinkIn      chan *protocol.Packet
	uplinkOut     chan *protocol.Packet
	ctx           context.Context
	cancel        context.CancelFunc
	muHello       sync.Mutex
	dataDir       string
	sendQ         *SendQueue
	wal           *wal.WAL
}

func (s *Session) handleUplinkPacket(pkt *protocol.Packet) error {
	switch pkt.Op {
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
		fmt.Printf("Received WAL ACK from uplink: %x\n", dedupKey)
	default:
		fmt.Printf("warn: unhandled uplink packet: Op=%d, Data=%x\n", pkt.Op, pkt.Data)
	}

	return nil
}

func makeDataDir(id []byte) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dataDir := fmt.Sprintf("%s/data/sessions/%02x/%030x", cwd, id[0:2], id[2:])
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", err
	}
	return dataDir, nil
}

func Run(ctx context.Context, conn ipc.Conn, hello *ipc.MessageBodyHelloIn) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dataDir, err := makeDataDir(hello.SessionID[:])
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
		Conn:          conn,
		SessionID:     hello.SessionID,
		SessionSecret: hello.SessionSecret,
		PlayerID:      hello.PlayerID,
		PlayerName:    hello.PlayerName,
		WorldID:       hello.WorldID,
		WalIndex:      hello.WalIndex,
		SeqGame:       seqGame,
		SeqNet:        seqNet,
		msgIn:         make(chan *ipc.Message, 16),
		msgOut:        make(chan []byte, 16),
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
	session.SendRaw(helloOut.Serialize())

	/* Start helper I/O goroutines */
	wg.Go(session.handleMsgIn)
	wg.Go(session.handleMsgOut)
	wg.Go(session.handleMsgLoop)
	wg.Go(session.handleUplink)

	/* Wait for cancellation */
	<-session.ctx.Done()
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
			err := s.Conn.Write(msg)
			if err != nil {
				fmt.Println("failed to write to IPC")
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (p *Session) SendRaw(data []byte) {
	select {
	case p.msgOut <- data:
	case <-p.ctx.Done():
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
	fmt.Printf("Added WAL entry to send queue: %+v\n", entry)

	/* Send the WAL entry to the uplink (optimization: don't wait for the uplink to read from the send queue) */
	pkt := protocol.Packet{Op: protocol.OpWal, Data: data}
	p.SendUplink(&pkt)

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
		entry.Item.Nonce = 0
		/* Todo: Handle nonce */
	default:
		return fmt.Errorf("unhandled WAL type: %d", w.Type)
	}

	return p.newWalEntry(&entry)
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

	fmt.Printf("Handling message from player %s: %+v\n", p.PlayerName, msg)

	switch msg.Op {
	case ipc.OpHello:
		helloIn, err := ipc.ParseMessageBodyHelloIn(msg.Payload)
		if err != nil {
			return err
		}
		if helloIn.Magic != [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00} {
			return fmt.Errorf("invalid magic in HELLO_IN message")
		}

		if helloIn.SessionID != p.SessionID || helloIn.SessionSecret != p.SessionSecret {
			return fmt.Errorf("invalid session ID or secret in HELLO_IN message")
		}

		if helloIn.PlayerID != p.PlayerID || helloIn.WorldID != p.WorldID {
			return fmt.Errorf("invalid player ID or world ID in HELLO_IN message")
		}

		p.PlayerName = helloIn.PlayerName
		p.WalIndex = helloIn.WalIndex

		replyBody := ipc.MessageBodyHelloOut{
			Magic:   [8]byte{'O', 'o', 'T', 'M', 'M', 0x7f, 0x01, 0x00},
			SeqGame: p.SeqGame,
			SeqNet:  p.SeqNet,
		}
		reply := ipc.Message{
			Seq:     0,
			Op:      ipc.OpHello,
			Payload: replyBody.Serialize(),
		}
		p.SendRaw(reply.Serialize())
	case ipc.OpWal:
		walMsg, err := ipc.ParseMessageBodyWalIn(msg.Payload)
		if err != nil {
			return err
		}
		return p.handleWalIn(walMsg)
	default:
		return fmt.Errorf("unhandled message opcode: %d", msg.Op)
	}
	return nil
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
