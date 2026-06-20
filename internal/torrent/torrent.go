package torrent

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"

	"bittorrent/internal/bencode"
)

type FileInfo struct {
	RawInfo []byte
	Info    map[string]bencode.Value

	Announce  string
	Name      string
	Length    int64
	PieceLen  int64
	Pieces    [][20]byte
	InfoHash  [20]byte
	NumPieces int
}

func ParseFile(path string) (*FileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("torrent: reading file: %v", err)
	}

	root, err := bencode.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("torrent: parsing bencode: %v", err)
	}

	dict, err := root.AsDict()
	if err != nil {
		return nil, fmt.Errorf("torrent: root is not a dict: %v", err)
	}

	tf := &FileInfo{
		Info: dict,
	}

	announceVal, ok := dict["announce"]
	if !ok {
		return nil, fmt.Errorf("torrent: missing 'announce' key")
	}
	tf.Announce, err = announceVal.AsString()
	if err != nil {
		return nil, fmt.Errorf("torrent: 'announce' is not a string: %v", err)
	}

	infoVal, ok := dict["info"]
	if !ok {
		return nil, fmt.Errorf("torrent: missing 'info' dict")
	}
	infoDict, err := infoVal.AsDict()
	if err != nil {
		return nil, fmt.Errorf("torrent: 'info' is not a dict: %v", err)
	}

	tf.Info = infoDict

	nameVal, ok := infoDict["name"]
	if !ok {
		return nil, fmt.Errorf("torrent: missing 'name' in info")
	}
	tf.Name, err = nameVal.AsString()
	if err != nil {
		return nil, fmt.Errorf("torrent: 'name' is not a string: %v", err)
	}

	lengthVal, ok := infoDict["length"]
	if !ok {
		return nil, fmt.Errorf("torrent: missing 'length' in info (multi-file not supported)")
	}
	tf.Length, err = lengthVal.AsInt()
	if err != nil {
		return nil, fmt.Errorf("torrent: 'length' is not an int: %v", err)
	}

	pieceLenVal, ok := infoDict["piece length"]
	if !ok {
		return nil, fmt.Errorf("torrent: missing 'piece length' in info")
	}
	tf.PieceLen, err = pieceLenVal.AsInt()
	if err != nil {
		return nil, fmt.Errorf("torrent: 'piece length' is not an int: %v", err)
	}

	piecesVal, ok := infoDict["pieces"]
	if !ok {
		return nil, fmt.Errorf("torrent: missing 'pieces' in info")
	}
	piecesStr, err := piecesVal.AsString()
	if err != nil {
		return nil, fmt.Errorf("torrent: 'pieces' is not a string: %v", err)
	}

	piecesRaw := []byte(piecesStr)
	if len(piecesRaw)%20 != 0 {
		return nil, fmt.Errorf("torrent: 'pieces' length %d is not a multiple of 20", len(piecesRaw))
	}
	tf.NumPieces = len(piecesRaw) / 20
	tf.Pieces = make([][20]byte, tf.NumPieces)
	for i := 0; i < tf.NumPieces; i++ {
		copy(tf.Pieces[i][:], piecesRaw[i*20:(i+1)*20])
	}

	rawInfo := infoVal.Encode()
	tf.RawInfo = rawInfo
	tf.InfoHash = sha1.Sum(rawInfo)

	return tf, nil
}

func (tf *FileInfo) GeneratePeerID() [20]byte {
	var id [20]byte
	prefix := "-GO0001-"
	copy(id[:], prefix)
	rand.Read(id[len(prefix):])
	return id
}
