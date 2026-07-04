package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/ipc"
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
	ctx           context.Context
	cancel        context.CancelFunc
	uplink        *Uplink
	muHello       sync.Mutex
}

func (s *Session) handleUplink() {
	var hello UplinkHello
	hello.SessionID = s.SessionID
	hello.SessionSecret = s.SessionSecret
	hello.PlayerID = s.PlayerID
	hello.WorldID = s.WorldID

	defer s.cancel()
	for s.ctx.Err() == nil {
		/* Capture HELLO state */
		s.muHello.Lock()
		hello.PlayerName = s.PlayerName
		hello.WalIndex = s.WalIndex
		s.muHello.Unlock()

		/* Process uplink */
		err := s.uplink.Run(&hello)
		if err != nil {
			fmt.Println("uplink error:", err)
			select {
			case <-time.After(5 * time.Second):
			case <-s.ctx.Done():
			}
		}
	}
}

func Run(ctx context.Context, conn ipc.Conn, hello *ipc.MessageBodyHelloIn) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
		uplink:        CreateUplink(ctx, "localhost:14236"),
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

func (p *Session) handleWalIn(wal *ipc.MessageBodyWalIn) error {
	switch wal.Type {
	case ipc.WalItem:
		item, err := ipc.ParseWalItemIn(wal.Data)
		if err != nil {
			return err
		}
		fmt.Printf("Received WAL item from player %s: %+v\n", p.PlayerName, item)
	default:
		return fmt.Errorf("unhandled WAL type: %d", wal.Type)
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
