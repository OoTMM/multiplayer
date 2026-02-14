package protocol

const (
	Version = 0x00000001
	Magic   = "OoTMM2\x00\xfe"

	NetOpNop   = 0x00
	NetOpHello = 0x01
	NetOpWal   = 0x02
)

func SerializeMessage(op uint8, payload []byte) []byte {
	buf := make([]byte, 1, 1+len(payload))
	buf[0] = op
	return append(buf, payload...)
}
