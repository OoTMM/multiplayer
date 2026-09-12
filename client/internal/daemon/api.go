package daemon

import (
	"net"
	"os"
	"time"

	"github.com/OoTMM/multiplayer/client/internal/util"
	"github.com/gofrs/flock"
)

func ensureDaemonStarted() error {
	os.MkdirAll(util.RunDir(), 0o700)

	lock := flock.New(daemonLockPath)
	ok, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	os.Remove(daemonSocketPath)
	lock.Close()
	os.Remove(daemonLockPath)
	return launchDaemon()
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

func Connect() (*DaemonConn, error) {
	/* Make sure the daemon is running */
	err := ensureDaemonStarted()
	if err != nil {
		return nil, err
	}

	/* Fast track: try to connect to the daemon first */
	conn, err := net.Dial("unix", daemonSocketPath)
	if err == nil {
		return createDaemonConn(conn), nil
	}

	/* Loop until the daemon socket becomes available or a timeout occurs */
	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return nil, err
		case <-tick:
			if _, err := os.Stat(daemonSocketPath); err == nil {
				conn, err = net.Dial("unix", daemonSocketPath)
				if err != nil {
					return nil, err
				}
				return createDaemonConn(conn), nil
			}
		}
	}
}
