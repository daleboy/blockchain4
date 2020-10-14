package blockchain4

import (
	"bytes"
	"encoding/binary"
	"log"
)

// IntToHex 将整型转为二进制数组
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}
