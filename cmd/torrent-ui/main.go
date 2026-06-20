package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"bittorrent/internal/api"
	"bittorrent/internal/session"
)

func main() {
	statePath := "torrent-ui-state.json"

	manager, err := session.LoadState(statePath)
	if err != nil {
		log.Fatalf("loading state: %v", err)
	}

	tmpl, err := api.ParseTemplates()
	if err != nil {
		log.Fatalf("parsing templates: %v", err)
	}

	server := api.NewServer(manager, tmpl)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("torrent-ui starting on http://localhost%s", addr)
	log.Printf("open your browser to http://localhost%s", addr)

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
