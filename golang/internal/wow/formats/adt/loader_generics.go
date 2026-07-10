package adt

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

// ReadStringBlock parses a null-terminated string block.
func ReadStringBlock(data *buffer.Buffer, chunkSize int) map[int]string {
	raw := data.ReadBuffer(buffer.ReadBufferOptions{Length: chunkSize, Wrap: false}).([]byte)
	entries := make(map[int]string)
	readOfs := 0
	for i := 0; i < chunkSize; i++ {
		if raw[i] == 0 {
			if readOfs == i {
				readOfs++
				continue
			}
			entries[readOfs] = strings.ReplaceAll(string(raw[readOfs:i]), "\x00", "")
			readOfs = i + 1
		}
	}
	return entries
}
