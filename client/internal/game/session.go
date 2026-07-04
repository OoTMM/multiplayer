package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/OoTMM/multiplayer/client/internal/ipc"
)

type Session struct {
	SessionID     [16]byte
	SessionSecret [8]byte
	Players       map[*SessionPlayer]struct{}
	muPlayers     sync.Mutex
}

type SessionPlayer struct {
	Session    *Session
	PlayerID   [16]byte
	PlayerName [8]byte
	WorldID    uint8
	WalIndex   uint32
	Conn       ipc.Conn
	SeqGame    uint32
	SeqNet     uint32
	msgIn      chan *ipc.Message
	msgOut     chan []byte
	ctx        context.Context
	cancel     context.CancelFunc
}

func OpenSession(sessionID [16]byte, sessionSecret [8]byte) *Session {
	return &Session{
		SessionID:     sessionID,
		SessionSecret: sessionSecret,
		Players:       make(map[*SessionPlayer]struct{}),
	}
}

func (p *SessionPlayer) handleMsgIn() {
	defer p.cancel()

	for {
		data, err := p.Conn.Read()
		if err != nil {
			if p.ctx.Err() == nil {
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
		case p.msgIn <- msg:
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *SessionPlayer) handleMsgOut() {
	defer p.cancel()

	for {
		select {
		case msg := <-p.msgOut:
			err := p.Conn.Write(msg)
			if err != nil {
				fmt.Println("failed to write to IPC")
				return
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *SessionPlayer) SendRaw(data []byte) {
	select {
	case p.msgOut <- data:
	case <-p.ctx.Done():
	}
}

func (p *SessionPlayer) handleWalIn(wal *ipc.MessageBodyWalIn) error {
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

func (p *SessionPlayer) handleMsg(msg *ipc.Message) error {
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

		if helloIn.SessionID != p.Session.SessionID || helloIn.SessionSecret != p.Session.SessionSecret {
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

func (p *SessionPlayer) handleMsgLoop() {
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

func (s *Session) ProcessPlayer(ctx context.Context, hello *ipc.MessageBodyHelloIn, conn ipc.Conn) {
	/* Create context & wg */
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	/* Generate random sequence numbers */
	randBytes := make([]byte, 8)
	rand.Read(randBytes)

	seqGame := binary.LittleEndian.Uint32(randBytes[0:4])
	seqNet := binary.LittleEndian.Uint32(randBytes[4:8])

	/* Create the player */
	player := &SessionPlayer{
		Session:    s,
		PlayerID:   hello.PlayerID,
		PlayerName: hello.PlayerName,
		WorldID:    hello.WorldID,
		WalIndex:   hello.WalIndex,
		Conn:       conn,
		SeqGame:    seqGame,
		SeqNet:     seqNet,
		msgIn:      make(chan *ipc.Message, 16),
		msgOut:     make(chan []byte, 16),
		ctx:        ctx,
		cancel:     cancel,
	}

	/* Add the player to the session */
	s.muPlayers.Lock()
	s.Players[player] = struct{}{}
	s.muPlayers.Unlock()

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
	player.SendRaw(helloOut.Serialize())

	/* Start helper I/O goroutines */
	wg.Go(player.handleMsgIn)
	wg.Go(player.handleMsgOut)
	wg.Go(player.handleMsgLoop)
	<-player.ctx.Done()
	player.Conn.Close()
	wg.Wait()

	/* Remove the player from the session */
	s.muPlayers.Lock()
	delete(s.Players, player)
	s.muPlayers.Unlock()
}
