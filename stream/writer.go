package stream

import (
	"encoding/binary"
	"github.com/seebs/nbt"
	"io"
	"math"
)

type Writer struct {
	w     io.Writer
	Error error
	buf   [32]byte
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w: w,
	}
}

func (w *Writer) WriteRaw(a []byte) *Writer {
	if _, err := w.w.Write(a); err != nil {
		w.Error = err
	}
	return w
}

func (w *Writer) WriteVarInt(a uint64) *Writer {
	n := binary.PutUvarint(w.buf[:], a)
	return w.WriteRaw(w.buf[:n])
}

func (w *Writer) WriteShort(a uint16) *Writer {
	binary.BigEndian.PutUint16(w.buf[:], a)
	return w.WriteRaw(w.buf[:2])
}

func (w *Writer) WriteInt(a uint32) *Writer {
	binary.BigEndian.PutUint32(w.buf[:], a)
	return w.WriteRaw(w.buf[:4])
}

func (w *Writer) WriteLong(a uint64) *Writer {
	binary.BigEndian.PutUint64(w.buf[:], a)
	return w.WriteRaw(w.buf[:8])
}

func (w *Writer) WriteFloat(a float32) *Writer {
	return w.WriteInt(math.Float32bits(a))
}

func (w *Writer) WriteDouble(a float64) *Writer {
	return w.WriteLong(math.Float64bits(a))
}

func (w *Writer) WriteString(a string) *Writer {
	return w.WriteVarInt(uint64(len(a))).WriteRaw([]byte(a))
}

func (w *Writer) WriteNbt(tag nbt.Compound) *Writer {
	if err := nbt.StoreUncompressed(w.w, tag, ""); err != nil {
		w.Error = err
	}
	return w
}
