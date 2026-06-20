package download

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"bittorrent/internal/peer"
	"bittorrent/internal/torrent"
)

const BlockSize = 1 << 14

type PeerAddr struct {
	IP   net.IP
	Port uint16
}

func (a PeerAddr) String() string {
	return net.JoinHostPort(a.IP.String(), fmt.Sprintf("%d", a.Port))
}

func Torrent(tf *torrent.FileInfo, addrs []PeerAddr, peerID [20]byte, output io.WriteSeeker, completed []int, log *log.Logger) error {
	done := make(map[int]bool)
	for _, p := range completed {
		done[p] = true
	}

	for _, addr := range addrs {
		log.Printf("connecting to peer %s", addr)

		conn, err := net.DialTimeout("tcp", addr.String(), 10*time.Second)
		if err != nil {
			log.Printf("failed to connect to %s: %v", addr, err)
			continue
		}

		err = downloadFromPeer(conn, tf, peerID, done, output, log)
		conn.Close()

		if err != nil {
			log.Printf("download from %s failed: %v", addr, err)
			continue
		}

		return nil
	}

	return fmt.Errorf("download: all peers failed")
}

func downloadFromPeer(conn net.Conn, tf *torrent.FileInfo, peerID [20]byte, done map[int]bool, output io.WriteSeeker, log *log.Logger) error {
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	log.Printf("performing handshake...")
	err := peer.DoHandshake(conn, tf.InfoHash, peerID)
	if err != nil {
		return fmt.Errorf("handshake: %v", err)
	}
	log.Printf("handshake successful")

	log.Printf("waiting for bitfield...")
	bitfieldMsg, err := peer.ReadMessage(conn)
	if err != nil {
		log.Printf("no bitfield received: %v", err)
	}
	if bitfieldMsg != nil && bitfieldMsg.ID == peer.Bitfield {
		log.Printf("received bitfield (%d bytes)", len(bitfieldMsg.Payload))
	}

	log.Printf("sending interested...")
	err = peer.WriteMessage(conn, peer.NewInterested())
	if err != nil {
		return fmt.Errorf("sending interested: %v", err)
	}

	log.Printf("waiting for unchoke...")
	err = waitForUnchoke(conn)
	if err != nil {
		return fmt.Errorf("waiting for unchoke: %v", err)
	}
	log.Printf("unchoked, starting download")

	for i := 0; i < tf.NumPieces; i++ {
		if done[i] {
			log.Printf("piece %d/%d already completed, skipping", i+1, tf.NumPieces)
			continue
		}

		log.Printf("downloading piece %d/%d...", i+1, tf.NumPieces)
		data, err := downloadPiece(conn, i, tf.PieceLen, tf.Length, tf.Pieces[i])
		if err != nil {
			return fmt.Errorf("piece %d: %v", i, err)
		}

		offset := int64(i) * tf.PieceLen
		_, err = output.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("seeking to piece %d: %v", i, err)
		}
		_, err = output.Write(data)
		if err != nil {
			return fmt.Errorf("writing piece %d: %v", i, err)
		}

		done[i] = true
		log.Printf("piece %d/%d downloaded and verified ✓", i+1, tf.NumPieces)
	}

	return nil
}

func waitForUnchoke(conn net.Conn) error {
	for {
		conn.SetDeadline(time.Now().Add(60 * time.Second))
		msg, err := peer.ReadMessage(conn)
		if err != nil {
			return err
		}
		if msg == nil {
			continue
		}
		switch msg.ID {
		case peer.Unchoke:
			return nil
		case peer.Choke:
		case peer.Have:
		case peer.Bitfield:
		case peer.Interested:
		case peer.NotInterested:
		default:
		}
	}
}

func downloadPiece(conn net.Conn, index int, pieceLen, fileLength int64, expectedHash [20]byte) ([]byte, error) {
	actualLength := pieceLen
	if int64(index+1)*pieceLen > fileLength {
		actualLength = fileLength - int64(index)*pieceLen
	}

	numBlocks := (actualLength + BlockSize - 1) / BlockSize
	blocks := make([][]byte, numBlocks)

	for b := int64(0); b < numBlocks; b++ {
		blockLen := BlockSize
		if b == numBlocks-1 {
			blockLen = int(actualLength - b*BlockSize)
		}
		begin := b * BlockSize

		req := peer.NewRequest(uint32(index), uint32(begin), uint32(blockLen))
		err := peer.WriteMessage(conn, req)
		if err != nil {
			return nil, fmt.Errorf("sending request: %v", err)
		}

		for {
			msg, err := peer.ReadMessage(conn)
			if err != nil {
				return nil, fmt.Errorf("reading piece response: %v", err)
			}
			if msg == nil {
				continue
			}
			if msg.ID == peer.Piece {
				pIdx := binary.BigEndian.Uint32(msg.Payload[0:4])
				pBegin := binary.BigEndian.Uint32(msg.Payload[4:8])
				if int(pIdx) != index || int(pBegin) != int(begin) {
					continue
				}
				blocks[b] = msg.Payload[8:]
				break
			}
		}
	}

	piece := make([]byte, 0, actualLength)
	for _, block := range blocks {
		piece = append(piece, block...)
	}

	hash := sha1.Sum(piece)
	if hash != expectedHash {
		return nil, fmt.Errorf("hash verification failed: expected %x, got %x", expectedHash, hash)
	}

	return piece, nil
}
