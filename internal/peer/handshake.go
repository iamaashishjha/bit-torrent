package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	HandshakeLen = 68

	ProtocolStr = "BitTorrent protocol"
)

type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

func DoHandshake(conn net.Conn, infoHash, peerID [20]byte) error {
	deadline := time.Now().Add(30 * time.Second)
	conn.SetDeadline(deadline)

	buf := make([]byte, HandshakeLen)
	buf[0] = 19
	copy(buf[1:20], ProtocolStr)
	copy(buf[28:48], infoHash[:])
	copy(buf[48:68], peerID[:])

	_, err := conn.Write(buf)
	if err != nil {
		return fmt.Errorf("handshake: write: %v", err)
	}

	resp := make([]byte, HandshakeLen)
	_, err = io.ReadFull(conn, resp)
	if err != nil {
		return fmt.Errorf("handshake: read: %v", err)
	}

	if resp[0] != 19 {
		return fmt.Errorf("handshake: bad protocol length: %d", resp[0])
	}
	if string(resp[1:20]) != ProtocolStr {
		return fmt.Errorf("handshake: bad protocol string: %q", string(resp[1:20]))
	}

	var respInfoHash [20]byte
	copy(respInfoHash[:], resp[28:48])
	if respInfoHash != infoHash {
		return fmt.Errorf("handshake: info hash mismatch:\n  sent: %x\n  recv: %x", infoHash, respInfoHash)
	}

	return nil
}

func EncodeHandshake(infoHash, peerID [20]byte) []byte {
	buf := make([]byte, HandshakeLen)
	buf[0] = 19
	copy(buf[1:20], ProtocolStr)
	copy(buf[28:48], infoHash[:])
	copy(buf[48:68], peerID[:])
	return buf
}

func DecodeHandshake(data []byte) (*Handshake, error) {
	if len(data) < HandshakeLen {
		return nil, fmt.Errorf("handshake: data too short: %d bytes", len(data))
	}
	if data[0] != 19 {
		return nil, fmt.Errorf("handshake: bad protocol length: %d", data[0])
	}
	if string(data[1:20]) != ProtocolStr {
		return nil, fmt.Errorf("handshake: bad protocol string: %q", string(data[1:20]))
	}

	var hs Handshake
	copy(hs.InfoHash[:], data[28:48])
	copy(hs.PeerID[:], data[48:68])
	return &hs, nil
}

func ReadHandshake(r io.Reader) (*Handshake, error) {
	data := make([]byte, HandshakeLen)
	_, err := io.ReadFull(r, data)
	if err != nil {
		return nil, fmt.Errorf("handshake: read: %v", err)
	}
	return DecodeHandshake(data)
}

func WriteHandshake(w io.Writer, infoHash, peerID [20]byte) error {
	data := EncodeHandshake(infoHash, peerID)
	_, err := w.Write(data)
	return err
}

func readInt(r io.Reader) (uint32, error) {
	var n uint32
	err := binary.Read(r, binary.BigEndian, &n)
	return n, err
}

func writeInt(w io.Writer, n uint32) error {
	return binary.Write(w, binary.BigEndian, n)
}
