package packet_marshal

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

// Packet represents a packet that can be marshaled.
// Compatible with Minecraft packet interface.
type Packet interface {
	Name() string
	Marshal(io protocol.IO)
}

// NamedPacket is a packet that also has an ID method for compatibility.
type NamedPacket interface {
	Packet
	ID() uint32
}

// Encode encodes a packet into its binary representation.
func Encode(pk interface{}) []byte {
	buf := &bytes.Buffer{}
	w := protocol.NewWriter(buf, 0)

	// 检查是否实现了Packet接口
	if packet, ok := pk.(Packet); ok {
		packet.Marshal(w)
	} else if namedPacket, ok := pk.(NamedPacket); ok {
		namedPacket.Marshal(w)
	} else {
		// 如果不是Packet类型，尝试断言为其他可能的类型
		// 这里可以根据需要添加更多类型检查
		return nil
	}

	return buf.Bytes()
}

// Decode decodes binary data into a packet using a factory function.
func Decode(data []byte, factory func() Packet) (Packet, error) {
	pk := factory()
	if pk == nil {
		return nil, fmt.Errorf("packet factory returned nil")
	}

	buf := bytes.NewReader(data)
	r := protocol.NewReader(buf, 0, true)
	pk.Marshal(r)
	return pk, nil
}

// FromBytes is a placeholder function for compatibility.
func FromBytes(data []byte) string {
	return string(data)
}

// ToString is a helper function to convert packet data to string.
func ToString(v any) (string, error) {
	// This is a simplified implementation
	// In real usage, this would serialize the value to JSON or other format
	switch val := v.(type) {
	case string:
		return val, nil
	case []byte:
		return string(val), nil
	default:
		// Convert to JSON string
		bs, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		return string(bs), nil
	}
}

// FromString is a helper function to convert string to packet data.
func FromString(s string) any {
	return s
}
