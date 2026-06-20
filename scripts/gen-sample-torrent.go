package main

import (
	"crypto/sha1"
	"fmt"
	"os"

	"bittorrent/internal/bencode"
	"bittorrent/internal/torrent"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: gen-sample-torrent <output-file> <content-file>\n")
		os.Exit(1)
	}
	outFile := os.Args[1]
	contentFile := os.Args[2]

	content, err := os.ReadFile(contentFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading content: %v\n", err)
		os.Exit(1)
	}

	pieceLen := int64(256 * 1024)
	if int64(len(content)) < pieceLen {
		pieceLen = int64(len(content))
	}

	torrentData := buildTorrent(content, pieceLen)

	infoHash, err := computeInfoHash(torrentData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "computing info hash: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile(outFile, torrentData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "writing torrent file: %v\n", err)
		os.Exit(1)
	}

	tf, err := torrent.ParseFile(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verifying torrent: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created torrent: %s\n", outFile)
	fmt.Printf("  Name:       %s\n", tf.Name)
	fmt.Printf("  Size:       %d bytes\n", tf.Length)
	fmt.Printf("  Piece len:  %d bytes\n", tf.PieceLen)
	fmt.Printf("  Pieces:     %d\n", tf.NumPieces)
	fmt.Printf("  Info hash:  %x\n", infoHash)
	fmt.Printf("  Tracker:    %s\n", tf.Announce)
	fmt.Printf("\n  Torrent verified successfully.\n")
}

func buildTorrent(content []byte, pieceLen int64) []byte {
	numPieces := (int64(len(content)) + pieceLen - 1) / pieceLen
	piecesData := make([]byte, 0, numPieces*20)
	for i := int64(0); i < numPieces; i++ {
		start := i * pieceLen
		end := start + pieceLen
		if end > int64(len(content)) {
			end = int64(len(content))
		}
		h := sha1.Sum(content[start:end])
		piecesData = append(piecesData, h[:]...)
	}

	announce := "http://tracker.example.com/announce"

	root := bencode.Value{
		Type: bencode.Dict,
		Dict: map[string]bencode.Value{
			"announce": {Type: bencode.String, Str: announce},
			"info": {
				Type: bencode.Dict,
				Dict: map[string]bencode.Value{
					"name":         {Type: bencode.String, Str: "sample.dat"},
					"length":       {Type: bencode.Int, Int: int64(len(content))},
					"piece length": {Type: bencode.Int, Int: pieceLen},
					"pieces":       {Type: bencode.String, Str: string(piecesData)},
				},
			},
			"creation date": {Type: bencode.Int, Int: 1234567890},
			"created by":    {Type: bencode.String, Str: "bittorrent-client"},
		},
	}

	return root.Encode()
}

func computeInfoHash(data []byte) ([20]byte, error) {
	root, err := bencode.Decode(data)
	if err != nil {
		return [20]byte{}, err
	}
	dict, err := root.AsDict()
	if err != nil {
		return [20]byte{}, err
	}
	infoVal, ok := dict["info"]
	if !ok {
		return [20]byte{}, fmt.Errorf("no info dict")
	}
	rawInfo := infoVal.Encode()
	return sha1.Sum(rawInfo), nil
}
