package protocol

import "encoding/binary"

// BytesToUint32 converts a byte slice to a uint32
func BytesToUint32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

// Uint32ToBytes converts a uint32 to a byte slice
func Uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}