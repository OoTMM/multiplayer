package protocol

type Opcode uint8

const (
	OpNOP      Opcode = 0x00
	OpHello    Opcode = 0x01
	OpWal      Opcode = 0x02
	OpWalAck   Opcode = 0x03
	OpPosition Opcode = 0x04
)
