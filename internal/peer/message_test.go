package peer

import (
	"bytes"
	"testing"
)

func TestSerializeKeepAlive(t *testing.T) {
	data := (*Message)(nil).Serialize()
	expected := []byte{0, 0, 0, 0}
	if !bytes.Equal(data, expected) {
		t.Fatalf("keep-alive: expected %v, got %v", expected, data)
	}
}

func TestSerializeInterested(t *testing.T) {
	msg := NewInterested()
	data := msg.Serialize()

	expected := []byte{0, 0, 0, 1, 2}
	if !bytes.Equal(data, expected) {
		t.Fatalf("interested: expected %v, got %v", expected, data)
	}
}

func TestReadWriteMessage(t *testing.T) {
	msg := NewHave(42)
	var buf bytes.Buffer
	err := WriteMessage(&buf, msg)
	if err != nil {
		t.Fatal(err)
	}

	read, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if read.ID != Have {
		t.Fatalf("expected Have message, got %d", read.ID)
	}

	index, err := ParseHave(read)
	if err != nil {
		t.Fatal(err)
	}
	if index != 42 {
		t.Fatalf("expected index 42, got %d", index)
	}
}

func TestReadMessageKeepAlive(t *testing.T) {
	buf := bytes.NewReader([]byte{0, 0, 0, 0})
	msg, err := ReadMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatal("expected nil for keep-alive")
	}
}

func TestRequestMessage(t *testing.T) {
	msg := NewRequest(0, 0, 16384)
	var buf bytes.Buffer
	err := WriteMessage(&buf, msg)
	if err != nil {
		t.Fatal(err)
	}

	read, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}

	index, begin, length, err := ParseRequest(read)
	if err != nil {
		t.Fatal(err)
	}
	if index != 0 || begin != 0 || length != 16384 {
		t.Fatalf("request: expected (0, 0, 16384), got (%d, %d, %d)", index, begin, length)
	}
}

func TestPieceMessage(t *testing.T) {
	blockData := []byte("test block data")
	payload := make([]byte, 8+len(blockData))
	payload[0] = 0
	payload[1] = 0
	payload[2] = 0
	payload[3] = 1
	payload[4] = 0
	payload[5] = 0
	payload[6] = 0
	payload[7] = 0
	copy(payload[8:], blockData)

	msg := &Message{ID: Piece, Payload: payload}
	var buf bytes.Buffer
	err := WriteMessage(&buf, msg)
	if err != nil {
		t.Fatal(err)
	}

	read, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}

	index, begin, data, err := ParsePiece(read)
	if err != nil {
		t.Fatal(err)
	}
	if index != 1 {
		t.Fatalf("expected index 1, got %d", index)
	}
	if begin != 0 {
		t.Fatalf("expected begin 0, got %d", begin)
	}
	if string(data) != string(blockData) {
		t.Fatalf("expected %q, got %q", blockData, data)
	}
}
