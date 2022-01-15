package packet

import (
	"bytes"
	"io"

	"github.com/laushunyu/real/constants"
	"github.com/laushunyu/real/stream"
	"github.com/laushunyu/real/utils"
	log "github.com/sirupsen/logrus"
)

type SPacket struct {
	// The meaning of a packet depends both on its packet ID and
	// the current state of the connection
	State constants.ConnState

	// Length of Packet ID + Data
	// Packets cannot be larger than 2097151 bytes
	Length   uint64
	PacketID uint64
	Data     []byte
}

type Packet struct {
	id  uint64
	buf *bytes.Buffer
	*stream.Writer
}

func (pkt Packet) Size() uint64 {
	return uint64(utils.UvarintLen(pkt.id) + pkt.buf.Len())
}

func (pkt Packet) WriteTo(w io.Writer) (n int64, err error) {
	log.Infof("send pkt %#2x to client with size = %d", pkt.id, pkt.Size())
	if err := stream.NewWriter(w).WriteVarInt(pkt.Size()).WriteVarInt(pkt.id).Error; err != nil {
		return 0, err
	}
	return pkt.buf.WriteTo(w)
}

func NewPacket(id uint64) Packet {
	buf := bytes.NewBuffer(nil)
	return Packet{
		id:     id,
		buf:    buf,
		Writer: stream.NewWriter(buf),
	}
}
