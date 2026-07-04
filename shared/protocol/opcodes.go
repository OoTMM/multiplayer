package protocol

type Opcode uint8

const (
	OpNOP   Opcode = 0x00
	OpHello Opcode = 0x01
)
