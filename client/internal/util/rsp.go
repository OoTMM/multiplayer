package util

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

type RSPConn struct {
	socket *net.TCPConn
	reader *bufio.Reader
	noAck  bool
}

func DialRSP(address string) (*RSPConn, error) {
	socket, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	tcp, ok := socket.(*net.TCPConn)
	if !ok {
		return nil, fmt.Errorf("failed to cast socket to TCPConn")
	}
	tcp.SetNoDelay(true)

	conn := &RSPConn{
		socket: tcp,
		reader: bufio.NewReader(socket),
		noAck:  false,
	}

	err = conn.init()
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *RSPConn) Close() {
	c.socket.CloseWrite()
	io.Copy(io.Discard, c.socket)
	c.socket.Close()
}

func (c *RSPConn) Detach() error {
	err := c.command([]byte("D"))
	if err != nil {
		return err
	}
	err = c.expectOk()
	if err != nil {
		return err
	}
	return nil
}

func (c *RSPConn) init() error {
	/* Initial ack */
	err := c.ack()
	if err != nil {
		return err
	}

	/* Enable no-ack mode */
	err = c.setNoAckMode()
	if err != nil {
		fmt.Println("Failed to set no-ack mode:", err)
	}

	fmt.Println("RSP connection initialized")

	return nil
}

func (c *RSPConn) setNoAckMode() error {
	err := c.command([]byte("QStartNoAckMode"))
	if err != nil {
		return err
	}
	err = c.expectOk()
	if err != nil {
		return err
	}
	c.noAck = true
	return nil
}

func (c *RSPConn) ack() error {
	if c.noAck {
		return nil
	}

	_, err := c.socket.Write([]byte("+"))
	return err
}

func (c *RSPConn) expectAck() error {
	if c.noAck {
		return nil
	}
	b, err := c.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != '+' {
		return fmt.Errorf("expected ack, got %v", b)
	}
	return nil
}

func (c *RSPConn) expectOk() error {
	data, err := c.readStructured()
	if err != nil {
		return err
	}
	if string(data) != "OK" {
		return fmt.Errorf("expected OK, got %s", string(data))
	}
	return nil
}

func (c *RSPConn) command(data []byte) error {
	/* Send the command */
	checksum := 0
	for _, b := range data {
		checksum += int(b)
	}
	checksum = checksum % 256

	packet := append([]byte{'$'}, data...)
	packet = append(packet, '#')
	packet = append(packet, []byte(fmt.Sprintf("%02x", checksum))...)

	_, err := c.socket.Write(packet)
	if err != nil {
		return err
	}

	return nil
}

func (c *RSPConn) readStructured() ([]byte, error) {
	err := c.expectAck()
	if err != nil {
		return nil, err
	}

	delim, err := c.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if delim != '$' {
		return nil, fmt.Errorf("expected '$', got %v", delim)
	}

	payload, err := c.reader.ReadBytes('#')
	if err != nil {
		return nil, err
	}
	payload = payload[:len(payload)-1]

	var sum [2]byte
	_, err = io.ReadFull(c.reader, sum[:])
	if err != nil {
		return nil, err
	}

	checksum := 0
	for _, b := range payload {
		checksum += int(b)
	}
	checksum = checksum % 256

	providedChecksum := 0
	_, err = fmt.Sscanf(string(sum[:]), "%02x", &providedChecksum)
	if err != nil {
		return nil, err
	}

	if checksum != providedChecksum {
		return nil, fmt.Errorf("checksum mismatch: expected %02x, got %02x", checksum, providedChecksum)
	}

	c.ack()
	return payload, nil
}
