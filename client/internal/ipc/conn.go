package ipc

type Conn interface {
	Close()
	Read() ([]byte, error)
	Write([]byte) error
}
