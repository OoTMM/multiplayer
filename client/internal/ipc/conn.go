package ipc

type Conn interface {
	Close()
	Read() ([]byte, error)
	Write([]byte) error
}

type ConnFactory interface {
	Open() (Conn, error)
}

type Poller interface {
	Poll() []ConnFactory
}
