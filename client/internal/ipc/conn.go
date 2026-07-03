package ipc

type RawConn interface {
	Close() error
	Read() ([]byte, error)
	Write([]byte) error
}
