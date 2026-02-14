package protocol

const (
	Version = 0x00000001
	Magic   = "OoTMM2\x00\xfe"

	NetOpNop   = 0x00
	NetOpHello = 0x01
)
