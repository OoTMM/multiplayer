package shared

import (
	"encoding/binary"
	"fmt"
)

type BytesReader struct {
	data []byte
	pos  int
	err  error
}

func NewBytesReader(data []byte) *BytesReader {
	return &BytesReader{data: data, pos: 0}
}

func (r *BytesReader) checkAvail(n int) bool {
	if r.err != nil {
		return false
	}
	if r.pos+n > len(r.data) {
		r.err = fmt.Errorf("byte reader: unexpected end-of-data")
		return false
	}
	return true
}

func (r *BytesReader) Err() error {
	return r.err
}

func (r *BytesReader) ReadBytes(n int) []byte {
	if !r.checkAvail(n) {
		return nil
	}
	result := r.data[r.pos : r.pos+n]
	r.pos += n
	return result
}

func (r *BytesReader) Read(out []byte) {
	n := len(out)
	if !r.checkAvail(n) {
		return
	}
	copy(out, r.data[r.pos:r.pos+n])
	r.pos += n
}

func (r *BytesReader) ReadUint8() uint8 {
	if !r.checkAvail(1) {
		return 0
	}
	val := r.data[r.pos]
	r.pos++
	return val
}

func (r *BytesReader) ReadUint16() uint16 {
	if !r.checkAvail(2) {
		return 0
	}
	val := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return val
}

func (r *BytesReader) ReadUint32() uint32 {
	if !r.checkAvail(4) {
		return 0
	}
	val := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return val
}

func (r *BytesReader) ReadUint64() uint64 {
	if !r.checkAvail(8) {
		return 0
	}
	val := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return val
}

func (r *BytesReader) Remaining() []byte {
	if r.err != nil {
		return nil
	}
	return r.data[r.pos:]
}

func (r *BytesReader) Len() int {
	if r.err != nil {
		return 0
	}
	return len(r.data) - r.pos
}
