package torrent

import (
	"crypto/sha1"
	"fmt"
	"os"
	"testing"
)

func createTestTorrent(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "test-*.torrent")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(data)
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func makeTorrent(announce, name string, length, pieceLen int64, piecesData []byte) []byte {
	data := []byte("d")
	data = append(data, []byte(fmt.Sprintf("8:announce%d:%s", len(announce), announce))...)
	data = append(data, []byte("4:info")...)
	data = append(data, []byte("d")...)

	infoKeys := []struct {
		key string
		val string
	}{
		{fmt.Sprintf("6:lengthi%de", length), ""},
		{fmt.Sprintf("4:name%d:%s", len(name), name), ""},
		{fmt.Sprintf("12:piece lengthi%de", pieceLen), ""},
		{fmt.Sprintf("6:pieces%d:", len(piecesData)), string(piecesData)},
	}

	for _, kv := range infoKeys {
		data = append(data, []byte(kv.key)...)
		if kv.val != "" {
			data = append(data, []byte(kv.val)...)
		}
	}

	data = append(data, []byte("e")...)
	data = append(data, []byte("e")...)
	return data
}

func TestParseTorrent(t *testing.T) {
	announce := "http://tracker.example.com/announce"
	content := "Hello World! This is a test torrent file."
	pieceLen := int64(len(content))
	hash := sha1.Sum([]byte(content))

	data := makeTorrent(announce, "test", int64(len(content)), pieceLen, hash[:])

	path := createTestTorrent(t, data)
	defer os.Remove(path)

	tf, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if tf.Announce != announce {
		t.Fatalf("announce: expected %q, got %q", announce, tf.Announce)
	}
	if tf.Name != "test" {
		t.Fatalf("name: expected 'test', got %q", tf.Name)
	}
	if tf.Length != int64(len(content)) {
		t.Fatalf("length: expected %d, got %d", len(content), tf.Length)
	}
	if tf.PieceLen != int64(len(content)) {
		t.Fatalf("piece length: expected %d, got %d", len(content), tf.PieceLen)
	}
	if tf.NumPieces != 1 {
		t.Fatalf("num pieces: expected 1, got %d", tf.NumPieces)
	}
}

func TestInfoHash(t *testing.T) {
	announce := "http://track.example.com/announce"
	title := "hello"
	hash := sha1.Sum([]byte{0})

	data := makeTorrent(announce, title, 1, 1, hash[:])

	path := createTestTorrent(t, data)
	defer os.Remove(path)

	tf, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(tf.InfoHash) != 20 {
		t.Fatalf("info hash should be 20 bytes, got %d", len(tf.InfoHash))
	}

	var zeroHash [20]byte
	if tf.InfoHash == zeroHash {
		t.Fatal("info hash should not be all zeros")
	}
}

func TestInfoHashConsistency(t *testing.T) {
	announce := "http://track.example.com/announce"
	hash := sha1.Sum([]byte{0})
	data := makeTorrent(announce, "test", 1, 1, hash[:])

	path := createTestTorrent(t, data)
	defer os.Remove(path)

	tf1, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	tf2, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if tf1.InfoHash != tf2.InfoHash {
		t.Fatalf("info hash should be deterministic:\n  run1: %x\n  run2: %x", tf1.InfoHash, tf2.InfoHash)
	}
}

func TestGeneratePeerID(t *testing.T) {
	announce := "http://track.example.com/announce"
	hash := sha1.Sum([]byte("test"))
	data := makeTorrent(announce, "test", 4, 4, hash[:])

	path := createTestTorrent(t, data)
	defer os.Remove(path)

	tf, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	id1 := tf.GeneratePeerID()
	id2 := tf.GeneratePeerID()

	if id1 == id2 {
		t.Fatal("peer IDs should be unique")
	}

	prefix := string(id1[:8])
	if prefix != "-GO0001-" {
		t.Fatalf("expected prefix '-GO0001-', got %q", prefix)
	}
}

func TestParseMultiPieceTorrent(t *testing.T) {
	announce := "http://track.example.com/announce"
	piece1Data := []byte("AAAAAAAAAAAAAAAAAAAA")
	piece2Data := []byte("BBBBBBBBBBBBBBBBBBBB")
	h1 := sha1.Sum(piece1Data)
	h2 := sha1.Sum(piece2Data)
	piecesData := append(h1[:], h2[:]...)

	data := makeTorrent(announce, "test", 40, 20, piecesData)

	path := createTestTorrent(t, data)
	defer os.Remove(path)

	tf, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if tf.NumPieces != 2 {
		t.Fatalf("expected 2 pieces, got %d", tf.NumPieces)
	}
	if tf.Pieces[0] != h1 {
		t.Fatalf("piece 0 hash mismatch")
	}
	if tf.Pieces[1] != h2 {
		t.Fatalf("piece 1 hash mismatch")
	}
}

func TestParseTorrentMissingKeys(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"no announce", []byte("d4:infodee")},
		{"no info", []byte("d8:announce0:e")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTestTorrent(t, tt.data)
			defer os.Remove(path)
			_, err := ParseFile(path)
			if err == nil {
				t.Fatal("expected error for missing key")
			}
		})
	}
}
