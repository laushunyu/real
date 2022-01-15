package generate

import (
	"bytes"

	"github.com/laushunyu/real/packet"
	"github.com/laushunyu/real/stream"
)

var blockNameId = map[string]uint64{
	"minecraft:air":                      0,
	"minecraft:dirt":                     48,
	"minecraft:bedrock":                  112,
	"minecraft:grass_block[snowy=false]": 32,
}

var plainChunkPalette = []string{"minecraft:air", "minecraft:dirt", "minecraft:bedrock", "minecraft:grass_block[snowy=false]"}

func GetPlainChunkDataPacket(x, z int32) packet.Packet {
	chunkBuf := bytes.NewBuffer(make([]byte, 0, 16*16*16*16))
	chunkWrt := stream.NewWriter(chunkBuf)

	var bitsPerBlock uint8 = 4 // min is 4, 2^4=16 type block

	// 1. write bits per block
	chunkWrt.WriteRaw([]byte{bitsPerBlock})

	// 2. write palette size
	chunkWrt.WriteVarInt(uint64(len(plainChunkPalette)))
	// 3. write palette block_ids
	for _, name := range plainChunkPalette {
		chunkWrt.WriteVarInt(blockNameId[name])
	}

	// 4. write block data size
	length := (16 * 16 * 16) * int(bitsPerBlock) / 64
	chunkWrt.WriteVarInt(uint64(length))
	// 5. write block data
	data := make([]byte, 16*16*16)
	// y<<8|z<<4|x
	for y := 0; y < 16; y++ {
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				if y == 0 {
					// bedrock
					data[(y<<8|z<<4|x)/2] |= byte(2 << ((x % 2) * 4))
				}
				if y >= 1 && y <= 2 {
					// dirt
					data[(y<<8|z<<4|x)/2] |= byte(1 << ((x % 2) * 4))
				}
				if y == 3 {
					// grass
					data[(y<<8|z<<4|x)/2] |= byte(3 << ((x % 2) * 4))
				}
			}
		}
	}
	chunkWrt.WriteRaw(data)

	// 6. write lights
	lights := make([]byte, 16*16*16/2)
	for i := range lights {
		lights[i] = 0xFF
	}
	chunkWrt.WriteRaw(lights)
	// Overworld skylight
	chunkWrt.WriteRaw(lights)

	// 7. write biomes
	biomes := make([]byte, 256)
	chunkWrt.WriteRaw(biomes)

	// new chunk packet
	chunkDataPkt := packet.NewPacket(0x20)
	chunkDataPkt.
		WriteInt(uint32(x)).
		WriteInt(uint32(z)).
		WriteRaw([]byte{1}).                 // ground up
		WriteVarInt(uint64(1)).              // bitmask, only 1 section returned
		WriteVarInt(uint64(chunkBuf.Len())). // size of byte array
		WriteRaw(chunkBuf.Bytes()).          // byte array
		WriteVarInt(0)
	return chunkDataPkt
}
