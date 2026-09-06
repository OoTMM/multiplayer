package daemon

import (
	"net"
)

type DaemonConn struct {
	conn   net.Conn
	chSend chan *Msg
}

func (conn *DaemonConn) Send(data *Msg) error {
	conn.chSend <- data
	return nil
}

func (d *DaemonConn) sendLoop() {
	for {
		data, ok := <-d.chSend
		if !ok {
			return
		}
		sendMsg(d.conn, data)
	}
}

func createDaemonConn(conn net.Conn) *DaemonConn {
	c := &DaemonConn{
		conn:   conn,
		chSend: make(chan *Msg, 128),
	}
	go c.sendLoop()
	return c
}

func (d *DaemonConn) Close() error {
	close(d.chSend)
	return d.conn.Close()
}
