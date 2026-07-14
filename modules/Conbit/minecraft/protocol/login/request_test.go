package login

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestEncodeRequestUsesHybridAuth2Outer(t *testing.T) {
	req := &request{
		Certificate: certificate{Chain: chain{"jwt-a", "jwt-b"}},
		Token:       "",
		RawToken:    "raw-token",
	}

	encoded := encodeRequest(req)
	buf := bytes.NewBuffer(encoded)

	var outerLength int32
	if err := binary.Read(buf, binary.LittleEndian, &outerLength); err != nil {
		t.Fatalf("read outer length: %v", err)
	}
	outerData := buf.Next(int(outerLength))

	var outer struct {
		Chain              []string `json:"chain"`
		Certificate        string   `json:"Certificate"`
		AuthenticationType uint8    `json:"AuthenticationType"`
		Token              string   `json:"Token"`
	}
	if err := json.Unmarshal(outerData, &outer); err != nil {
		t.Fatalf("decode outer json: %v", err)
	}
	if outer.AuthenticationType != 2 {
		t.Fatalf("unexpected auth type: %d", outer.AuthenticationType)
	}
	if len(outer.Chain) != 2 {
		t.Fatalf("unexpected outer chain length: %d", len(outer.Chain))
	}
	if outer.Certificate == "" {
		t.Fatal("expected certificate string to be populated")
	}
}
