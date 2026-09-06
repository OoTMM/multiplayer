package daemon

import (
	"net"
	"os"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/util"
)

type DaemonConn struct {
	conn net.Conn
}

func Connect() (*DaemonConn, error) {
	/* Fast track: try to connect to the daemon first */
	conn, err := net.Dial("unix", daemonSocketPath)
	if err == nil {
		return &DaemonConn{conn: conn}, nil
	}

	/* The connection failed, try to launch the daemon */
	err = launchDaemon()
	if err != nil {
		return nil, err
	}

	/* Wait for the daemon to start */
	time.Sleep(2 * time.Second)

	/* Try to connect again after launching the daemon */
	conn, err = net.Dial("unix", daemonSocketPath)
	if err != nil {
		return nil, err
	}
	return &DaemonConn{conn: conn}, nil
}

/* Re-start the process, passing a special flag */
func launchDaemon() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	err = util.StartDetachedProcess(execPath, []string{execPath, "--daemon"})
	if err != nil {
		return err
	}
	return nil
}

func (d *DaemonConn) Close() error {
	return d.conn.Close()
}
