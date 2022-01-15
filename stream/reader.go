package stream

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
)

type Reader struct {
	r interface {
		io.Reader
		io.ByteReader
	}
}

func NewReader(r io.Reader) *Reader {
	return &Reader{
		r: bufio.NewReader(r),
	}
}

func (r *Reader) ReadRaw(count int) ([]byte, error) {
	if count <= 0 {
		return []byte{}, nil
	}
	buf := make([]byte, count)
	_, err := io.ReadFull(r.r, buf)
	return buf, err
}

func (r *Reader) ReadByte() (byte, error) {
	raw, err := r.ReadRaw(1)
	if err != nil {
		return 0, err
	}
	return raw[0], err
}

func (r *Reader) ReadBoolean() (bool, error) {
	b, err := r.ReadByte()
	if err != nil {
		return false, err
	}
	return b != 0, err
}

func (r *Reader) ReadString() (string, error) {
	length, err := r.ReadVarInt()
	if err != nil {
		return "", err
	}

	raw, err := r.ReadRaw(int(length))
	return string(raw), err
}

func (r *Reader) ReadShort() (uint16, error) {
	buf, err := r.ReadRaw(2)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(buf), nil
}

func (r *Reader) ReadInt() (uint32, error) {
	buf, err := r.ReadRaw(4)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buf), nil
}

func (r *Reader) ReadLong() (uint64, error) {
	buf, err := r.ReadRaw(8)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(buf), nil
}

func (r *Reader) ReadVarInt() (uint64, error) {
	return binary.ReadUvarint(r.r)
}

func (r *Reader) ReadDouble() (float64, error) {
	a, err := r.ReadLong()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(a), nil
}

func (r *Reader) ReadFloat() (float32, error) {
	a, err := r.ReadInt()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(a), nil
}
