package tracker

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestParseCompactPeers(t *testing.T) {
	data := make([]byte, 12)

	copy(data[0:4], []byte{192, 168, 1, 1})
	binary.BigEndian.PutUint16(data[4:6], 6881)

	copy(data[6:10], []byte{10, 0, 0, 1})
	binary.BigEndian.PutUint16(data[10:12], 6882)

	peers, err := parseCompactPeers(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}

	if !peers[0].IP.Equal(net.IP{192, 168, 1, 1}) {
		t.Fatalf("peer 0 IP: expected 192.168.1.1, got %s", peers[0].IP)
	}
	if peers[0].Port != 6881 {
		t.Fatalf("peer 0 port: expected 6881, got %d", peers[0].Port)
	}

	if !peers[1].IP.Equal(net.IP{10, 0, 0, 1}) {
		t.Fatalf("peer 1 IP: expected 10.0.0.1, got %s", peers[1].IP)
	}
	if peers[1].Port != 6882 {
		t.Fatalf("peer 1 port: expected 6882, got %d", peers[1].Port)
	}
}

func TestParseCompactPeersEmpty(t *testing.T) {
	peers, err := parseCompactPeers([]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(peers))
	}
}

func TestParseCompactPeersInvalidLength(t *testing.T) {
	_, err := parseCompactPeers([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for invalid peer data length")
	}
}

func TestParseResponse(t *testing.T) {
	body := []byte("d8:intervali1800e5:peers12:\x01\x02\x03\x04\x1a\xe1\x05\x06\x07\x08\x1a\xe2e")
	resp, err := parseResponse(body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Interval != 1800 {
		t.Fatalf("interval: expected 1800, got %d", resp.Interval)
	}
	if len(resp.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(resp.Peers))
	}
	if resp.Peers[0].Port != 6881 {
		t.Fatalf("peer 0 port: expected 6881, got %d", resp.Peers[0].Port)
	}
	if resp.Peers[1].Port != 6882 {
		t.Fatalf("peer 1 port: expected 6882, got %d", resp.Peers[1].Port)
	}
}

func TestParseResponseFailure(t *testing.T) {
	body := []byte("d14:failure reason20:test failure messagee")
	_, err := parseResponse(body)
	if err == nil {
		t.Fatal("expected error for failure response")
	}
	if err.Error() != "tracker: test failure message" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseResponseNoPeers(t *testing.T) {
	body := []byte("d8:intervali1800e5:peers0:e")
	_, err := parseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseResponseMissingPeers(t *testing.T) {
	body := []byte("d8:intervali1800ee")
	_, err := parseResponse(body)
	if err == nil {
		t.Fatal("expected error for missing peers")
	}
}
