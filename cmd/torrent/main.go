package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"bittorrent/internal/download"
	"bittorrent/internal/storage"
	"bittorrent/internal/torrent"
	"bittorrent/internal/tracker"
)

func main() {
	torrentPath := flag.String("torrent", "", "path to .torrent file")
	outputDir := flag.String("out", ".", "output directory")
	port := flag.Uint("port", 6881, "listening port for tracker announces")
	flag.Parse()

	if *torrentPath == "" {
		fmt.Fprintf(os.Stderr, "usage: torrent --torrent <file.torrent> --out <output-dir>\n")
		os.Exit(1)
	}

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Printf("parsing torrent file: %s", *torrentPath)

	tf, err := torrent.ParseFile(*torrentPath)
	if err != nil {
		log.Fatalf("failed to parse torrent: %v", err)
	}

	log.Printf("tracker: %s", tf.Announce)
	log.Printf("name: %s", tf.Name)
	log.Printf("size: %d bytes", tf.Length)
	log.Printf("piece length: %d bytes", tf.PieceLen)
	log.Printf("pieces: %d", tf.NumPieces)
	log.Printf("info hash: %x", tf.InfoHash)

	peerID := tf.GeneratePeerID()
	log.Printf("peer ID: %x", peerID)

	log.Printf("contacting tracker...")
	resp, err := tracker.Announce(tf, peerID, uint16(*port))
	if err != nil {
		log.Fatalf("tracker announce failed: %v", err)
	}

	log.Printf("tracker interval: %ds", resp.Interval)
	log.Printf("peers discovered: %d", len(resp.Peers))
	for i, p := range resp.Peers {
		log.Printf("  peer %d: %s:%d", i, p.IP, p.Port)
	}

	if len(resp.Peers) == 0 {
		log.Fatalf("no peers returned from tracker")
	}

	outPath := filepath.Join(*outputDir, tf.Name)
	statePath := outPath + ".resume.json"

	completed, err := storage.LoadResumeState(statePath)
	if err != nil {
		log.Printf("warning: could not load resume state: %v", err)
	}
	if completed == nil {
		completed = []int{}
	}
	log.Printf("resume state: %d pieces already completed", len(completed))

	var outputFile io.WriteSeeker
	if storage.FileExists(outPath) && len(completed) > 0 {
		outputFile, err = storage.OpenFile(outPath)
		if err != nil {
			log.Fatalf("opening existing output file: %v", err)
		}
		log.Printf("resuming download to existing file: %s", outPath)
	} else {
		outputFile, err = storage.SaveFile(outPath, tf.Length)
		if err != nil {
			log.Fatalf("creating output file: %v", err)
		}
		log.Printf("created output file: %s (%d bytes)", outPath, tf.Length)
	}

	addrs := make([]download.PeerAddr, len(resp.Peers))
	for i, p := range resp.Peers {
		addrs[i] = download.PeerAddr{IP: p.IP, Port: p.Port}
	}

	err = download.Torrent(tf, addrs, peerID, outputFile, completed, log.Default())
	if err != nil {
		log.Fatalf("download failed: %v", err)
	}

	err = storage.SaveResumeState(statePath, completed)
	if err != nil {
		log.Printf("warning: saving resume state: %v", err)
	}

	log.Printf("download complete: %s", outPath)
}
