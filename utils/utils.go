package utils

import "encoding/binary"

var varIntLenBuf = [10]byte{}

func UvarintLen(num uint64) (size int) {
	return binary.PutUvarint(varIntLenBuf[:], num)
}
