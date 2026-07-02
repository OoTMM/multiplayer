package emulators

import (
	"context"
	"fmt"

	"github.com/OoTMM/multiplayer/client/internal/util"
)

type GDBConnector struct {
	ctx  context.Context
	conn *util.RSPConn
}

func ConnectGDB(ctx context.Context) Connector {
	conn, err := util.DialRSP("localhost:9123")
	if err != nil {
		return nil
	}

	connector := &GDBConnector{
		ctx:  ctx,
		conn: conn,
	}

	return connector
}

func (g *GDBConnector) Close() {
	err := g.conn.Detach()
	if err != nil {
		fmt.Println("Failed to detach:", err)
	}
	g.conn.Close()
	fmt.Println("GDB connector closed")
}
