package command

import (
	"encoding/binary"
	"io"
)

type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}

func writeString(writer io.Writer, str string) error {
	data := []byte(str)
	u32_len := uint32(len(data))
	binary.Write(writer, binary.BigEndian, u32_len)
	_, err := writer.Write(data)
	if err != nil {
		return err
	}
	return err
	// data := []byte(str)
	// u32_len := uint32(len(data))
	// buf := make([]byte, 4)
	//
	// binary.BigEndian.PutUint32(buf, u32_len)
	// _, err := writer.Write(buf)
	// if err != nil {
	// 	return err
	// }
	// return err
}

func readString(reader io.Reader) (string, error) {
	var u32_len uint32
	binary.Read(reader, binary.BigEndian, &u32_len)

	data := make([]byte, u32_len)
	_, err := io.ReadFull(reader, data)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var WriteString = writeString
var ReadString = readString
