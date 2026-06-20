package api

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bittorrent/internal/security"
	"bittorrent/internal/session"
	"bittorrent/internal/torrent"
)

type Server struct {
	manager *session.Manager
	mux     *http.ServeMux
	tmpl    *template.Template
}

func NewServer(manager *session.Manager, tmpl *template.Template) *Server {
	s := &Server{
		manager: manager,
		mux:     http.NewServeMux(),
		tmpl:    tmpl,
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.handleDashboard)
	s.mux.HandleFunc("GET /api/torrents", s.handleListTorrents)
	s.mux.HandleFunc("POST /api/torrents", s.handleAddTorrent)
	s.mux.HandleFunc("GET /api/torrents/{id}", s.handleGetTorrent)
	s.mux.HandleFunc("POST /api/torrents/{id}/start", s.handleStartTorrent)
	s.mux.HandleFunc("POST /api/torrents/{id}/pause", s.handlePauseTorrent)
	s.mux.HandleFunc("POST /api/torrents/{id}/resume", s.handleResumeTorrent)
	s.mux.HandleFunc("DELETE /api/torrents/{id}", s.handleRemoveTorrent)
	s.mux.HandleFunc("GET /api/torrents/{id}/peers", s.handleGetPeers)
	s.mux.HandleFunc("GET /api/torrents/{id}/trackers", s.handleGetTrackers)
	s.mux.HandleFunc("GET /api/torrents/{id}/security-report", s.handleGetSecurity)
	s.mux.HandleFunc("GET /api/events", s.handleSSE)
	s.mux.HandleFunc("GET /add", s.handleAddPage)
	s.mux.HandleFunc("GET /torrent/{id}", s.handleDetailPage)
	s.mux.HandleFunc("GET /torrent/{id}/peers", s.handlePeersPage)
	s.mux.HandleFunc("GET /torrent/{id}/trackers", s.handleTrackersPage)
	s.mux.HandleFunc("GET /torrent/{id}/security", s.handleSecurityPage)
	s.mux.HandleFunc("GET /settings", s.handleSettingsPage)
	s.mux.HandleFunc("POST /api/settings", s.handleUpdateSettings)
	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	fs := http.FileServer(http.Dir("web/static"))
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.render(w, "dashboard", map[string]interface{}{
		"torrents": s.manager.List(),
	})
}

func (s *Server) handleListTorrents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.manager.List())
}

func (s *Server) handleAddTorrent(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("torrent")
	if err != nil {
		http.Error(w, "missing torrent file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp("", "upload-*.torrent")
	if err != nil {
		http.Error(w, "failed to save upload", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		tmpFile.Close()
		http.Error(w, "failed to read upload", http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	outputDir := r.FormValue("output_dir")
	if outputDir == "" {
		outputDir = s.manager.Settings().DefaultDir
	}

	ts, err := s.manager.AddTorrentFile(tmpFile.Name(), outputDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	statePath := filepath.Join(ts.OutputDir, ts.Name+".torrent")
	os.Rename(tmpFile.Name(), statePath)
	ts.TorrentPath = statePath

	s.manager.Start(ts.ID)

	w.Header().Set("HX-Redirect", "/torrent/"+ts.ID)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleGetTorrent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, ts)
}

func (s *Server) handleStartTorrent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.manager.Start(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"status": "started"})
}

func (s *Server) handlePauseTorrent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.manager.Pause(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"status": "paused"})
}

func (s *Server) handleResumeTorrent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.manager.Resume(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"status": "resumed"})
}

func (s *Server) handleRemoveTorrent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	deleteFiles := r.URL.Query().Get("delete_files") == "true"
	if err := s.manager.Remove(id, deleteFiles); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetPeers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ts.RLock()
	peers := ts.Peers
	ts.RUnlock()
	writeJSON(w, peers)
}

func (s *Server) handleGetTrackers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ts.RLock()
	tracker := ts.Tracker
	ts.RUnlock()
	writeJSON(w, tracker)
}

func (s *Server) handleGetSecurity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ts.RLock()
	torrentPath := ts.TorrentPath
	ts.RUnlock()

	tf, err := torrent.ParseFile(torrentPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	report := security.ScanTorrent(tf)
	writeJSON(w, report)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.manager.SSE().Subscribe()
	defer s.manager.SSE().Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) handleAddPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "add-torrent", map[string]interface{}{
		"settings": s.manager.Settings(),
	})
}

func (s *Server) handleDetailPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.NotFound(w, r)
		return
	}

	tf, err := torrent.ParseFile(ts.TorrentPath)
	var report *security.Report
	if err == nil {
		report = security.ScanTorrent(tf)
	}

	s.render(w, "detail", map[string]interface{}{
		"torrent": ts,
		"report":  report,
	})
}

func (s *Server) handlePeersPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.NotFound(w, r)
		return
	}
	ts.RLock()
	peers := ts.Peers
	ts.RUnlock()
	s.render(w, "peers", map[string]interface{}{
		"torrent": ts,
		"peers":   peers,
	})
}

func (s *Server) handleTrackersPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.NotFound(w, r)
		return
	}
	ts.RLock()
	tracker := ts.Tracker
	ts.RUnlock()
	s.render(w, "trackers", map[string]interface{}{
		"torrent": ts,
		"tracker": tracker,
	})
}

func (s *Server) handleSecurityPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ts := s.manager.Get(id)
	if ts == nil {
		http.NotFound(w, r)
		return
	}

	tf, err := torrent.ParseFile(ts.TorrentPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	report := security.ScanTorrent(tf)
	s.render(w, "security", map[string]interface{}{
		"torrent": ts,
		"report":  report,
	})
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings", map[string]interface{}{
		"settings": s.manager.Settings(),
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.manager.Settings())
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings session.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.manager.UpdateSettings(settings)
	writeJSON(w, map[string]string{"status": "saved"})
}

func (s *Server) render(w http.ResponseWriter, page string, data map[string]interface{}) {
	data["page"] = page

	if pageData, ok := data["torrent"]; ok {
		if ts, ok := pageData.(*session.TorrentSession); ok {
			ts.RLock()
			data["torrent_id"] = ts.ID
			data["torrent_name"] = ts.Name
			data["torrent_state"] = ts.State.String()
			ts.RUnlock()
		}
	}

	err := s.tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func humanSpeed(bps float64) string {
	if bps < 0 {
		return "0 B/s"
	}
	const unit = 1024
	if bps < unit {
		return fmt.Sprintf("%.0f B/s", bps)
	}
	div, exp := float64(unit), 0
	for n := bps / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bps/div, "KMGTPE"[exp])
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatETA(seconds float64) string {
	if seconds <= 0 || seconds > 1e9 {
		return "∞"
	}
	d := int(seconds)
	h := d / 3600
	m := (d % 3600) / 60
	s := d % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func percentClass(pct float64) string {
	if pct >= 100 {
		return "progress-complete"
	}
	if pct > 0 {
		return "progress-active"
	}
	return "progress-idle"
}

func stateLabel(state session.State) string {
	return strings.ToUpper(state.String())
}

func timeAgo(ts string) string {
	return ts
}

var FuncMap = template.FuncMap{
	"humanSpeed":   humanSpeed,
	"formatSize":   formatSize,
	"formatETA":    formatETA,
	"percentClass": percentClass,
	"stateLabel":   stateLabel,
	"timeAgo":      timeAgo,
	"upper":        strings.ToUpper,
	"safeHTML":     func(s string) template.HTML { return template.HTML(s) },
	"attr":         html.EscapeString,
}

func ParseTemplates() (*template.Template, error) {
	return template.New("").Funcs(FuncMap).ParseGlob("web/templates/*.html")
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
