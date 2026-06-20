package peer

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeHandshake(t *testing.T) {
	var infoHash, peerID [20]byte
	copy(infoHash[:], []byte("12345678901234567890"))
	copy(peerID[:], []byte("ABCDEFGHIJKLMNOPQRST"))

	data := EncodeHandshake(infoHash, peerID)

	if len(data) != HandshakeLen {
		t.Fatalf("handshake length: expected %d, got %d", HandshakeLen, len(data))
	}

	hs, err := DecodeHandshake(data)
	if err != nil {
		t.Fatal(err)
	}

	if hs.InfoHash != infoHash {
		t.Fatalf("info hash mismatch:\n  expected: %x\n  got:      %x", infoHash, hs.InfoHash)
	}
	if hs.PeerID != peerID {
		t.Fatalf("peer ID mismatch:\n  expected: %x\n  got:      %x", peerID, hs.PeerID)
	}
}

func TestDecodeHandshakeTooShort(t *testing.T) {
	_, err := DecodeHandshake([]byte{19, 'B', 'i', 't'})
	if err == nil {
		t.Fatal("expected error for short handshake")
	}
}

func TestDecodeHandshakeBadProtocolLength(t *testing.T) {
	data := make([]byte, HandshakeLen)
	data[0] = 99
	_, err := DecodeHandshake(data)
	if err == nil {
		t.Fatal("expected error for bad protocol length")
	}
}

func TestDecodeHandshakeBadProtocol(t *testing.T) {
	data := make([]byte, HandshakeLen)
	data[0] = 19
	copy(data[1:20], []byte("NOT a protocol str"))
	_, err := DecodeHandshake(data)
	if err == nil {
		t.Fatal("expected error for bad protocol string")
	}
}

func TestHandshakeProtocolString(t *testing.T) {
	var infoHash, peerID [20]byte
	data := EncodeHandshake(infoHash, peerID)

	if data[0] != 19 {
		t.Fatalf("expected protocol length 19, got %d", data[0])
	}
	if string(data[1:20]) != "BitTorrent protocol" {
		t.Fatalf("expected 'BitTorrent protocol', got %q", string(data[1:20]))
	}
}

func TestHandshakeHasReservedBytes(t *testing.T) {
	var infoHash, peerID [20]byte
	data := EncodeHandshake(infoHash, peerID)

	for i := 20; i < 28; i++ {
		if data[i] != 0 {
			t.Fatalf("reserved byte %d should be 0, got %d", i, data[i])
		}
	}
}

func TestReadWriteHandshake(t *testing.T) {
	var infoHash, peerID [20]byte
	copy(infoHash[:], []byte("12345678901234567890"))
	copy(peerID[:], []byte("ABCDEFGHIJKLMNOPQRST"))

	var buf bytes.Buffer
	err := WriteHandshake(&buf, infoHash, peerID)
	if err != nil {
		t.Fatal(err)
	}

	hs, err := ReadHandshake(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if hs.InfoHash != infoHash {
		t.Fatalf("info hash mismatch")
	}
	if hs.PeerID != peerID {
		t.Fatalf("peer ID mismatch")
	}
}
