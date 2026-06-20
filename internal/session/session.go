package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bittorrent/internal/download"
	"bittorrent/internal/storage"
	"bittorrent/internal/torrent"
	"bittorrent/internal/tracker"
)

type State int

const (
	StateQueued State = iota
	StateDownloading
	StateSeeding
	StatePaused
	StateCompleted
	StateError
)

func (s State) String() string {
	switch s {
	case StateQueued:
		return "queued"
	case StateDownloading:
		return "downloading"
	case StateSeeding:
		return "seeding"
	case StatePaused:
		return "paused"
	case StateCompleted:
		return "completed"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

type Settings struct {
	DefaultDir      string  `json:"default_dir"`
	Port            int     `json:"port"`
	MaxActive       int     `json:"max_active"`
	MaxDownloadSpeed float64 `json:"max_download_speed"`
	MaxUploadSpeed  float64 `json:"max_upload_speed"`
	EnablePeers     bool    `json:"enable_peers"`
	EnableSeeding   bool    `json:"enable_seeding"`
}

type PeerInfo struct {
	IP           string `json:"ip"`
	Port         int    `json:"port"`
	Choked       bool   `json:"choked"`
	Interested   bool   `json:"interested"`
	Progress     float64 `json:"progress"`
	LastActive   string `json:"last_active"`
}

type TrackerInfo struct {
	AnnounceURL  string `json:"announce_url"`
	LastAnnounce string `json:"last_announce"`
	NextAnnounce string `json:"next_announce"`
	Seeders      int    `json:"seeders"`
	Leechers     int    `json:"leechers"`
	Completed    int    `json:"completed"`
	Status       string `json:"status"`
}

type TorrentSession struct {
	ID              string    `json:"id"`
	InfoHash        string    `json:"info_hash"`
	Name            string    `json:"name"`
	TotalSize       int64     `json:"total_size"`
	Downloaded      int64     `json:"downloaded"`
	Uploaded        int64     `json:"uploaded"`
	State           State     `json:"state"`
	Progress        float64   `json:"progress"`
	DownloadSpeed   float64   `json:"download_speed"`
	UploadSpeed     float64   `json:"upload_speed"`
	ETA             float64   `json:"eta"`
	Ratio           float64   `json:"ratio"`
	Peers           []PeerInfo `json:"peers"`
	Tracker         TrackerInfo `json:"tracker"`
	CompletedPieces []int     `json:"completed_pieces"`
	NumPieces       int       `json:"num_pieces"`
	Error           string    `json:"error,omitempty"`
	OutputDir       string    `json:"output_dir"`
	OutputPath      string    `json:"output_path"`
	TorrentPath     string    `json:"torrent_path"`

	tf     *torrent.FileInfo
	peerID [20]byte
	file   *os.File
	cancel context.CancelFunc
	mu     sync.RWMutex

	speedTracker SpeedTracker
}

type SpeedTracker struct {
	mu      sync.Mutex
	samples []speedSample
}

type speedSample struct {
	time  time.Time
	bytes int64
}

func (st *SpeedTracker) Record(bytes int64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now()
	st.samples = append(st.samples, speedSample{time: now, bytes: bytes})
	cutoff := now.Add(-10 * time.Second)
	j := 0
	for i, s := range st.samples {
		if s.time.After(cutoff) {
			st.samples = st.samples[i:]
			break
		}
		j = i
	}
	if j > 0 && j < len(st.samples) {
		st.samples = st.samples[j:]
	}
}

func (st *SpeedTracker) Rate() float64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.samples) < 2 {
		return 0
	}
	first := st.samples[0]
	last := st.samples[len(st.samples)-1]
	elapsed := last.time.Sub(first.time).Seconds()
	if elapsed <= 0 {
		return 0
	}
	total := last.bytes - first.bytes
	return float64(total) / elapsed
}

type Manager struct {
	mu       sync.RWMutex
	torrents map[string]*TorrentSession
	settings Settings
	sse      *SSEHub
	stopCh   chan struct{}
}

type SSEHub struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

func NewSSEHub() *SSEHub {
	return &SSEHub{clients: make(map[chan string]struct{})}
}

func (h *SSEHub) Subscribe() chan string {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *SSEHub) Unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *SSEHub) Broadcast(data string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
		}
	}
}

type persistState struct {
	Torrents []persistTorrent `json:"torrents"`
	Settings Settings         `json:"settings"`
}

type persistTorrent struct {
	ID              string   `json:"id"`
	TorrentPath     string   `json:"torrent_path"`
	OutputDir       string   `json:"output_dir"`
	CompletedPieces []int    `json:"completed_pieces"`
	Downloaded      int64    `json:"downloaded"`
	Uploaded        int64    `json:"uploaded"`
}

func NewManager(settings Settings) *Manager {
	return &Manager{
		torrents: make(map[string]*TorrentSession),
		settings: settings,
		sse:      NewSSEHub(),
		stopCh:   make(chan struct{}),
	}
}

func (m *Manager) SSE() *SSEHub {
	return m.sse
}

func (m *Manager) Settings() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings
}

func (m *Manager) UpdateSettings(s Settings) {
	m.mu.Lock()
	m.settings = s
	m.mu.Unlock()
	m.saveState()
}

func (m *Manager) List() []*TorrentSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*TorrentSession, 0, len(m.torrents))
	for _, ts := range m.torrents {
		result = append(result, ts)
	}
	return result
}

func (m *Manager) Get(id string) *TorrentSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.torrents[id]
}

func (m *Manager) AddTorrentFile(torrentPath, outputDir string) (*TorrentSession, error) {
	tf, err := torrent.ParseFile(torrentPath)
	if err != nil {
		return nil, fmt.Errorf("session: parsing torrent: %v", err)
	}

	infoHash := fmt.Sprintf("%x", tf.InfoHash)

	m.mu.Lock()
	if _, exists := m.torrents[infoHash]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("session: torrent already added: %s", infoHash)
	}

	if outputDir == "" {
		outputDir = m.settings.DefaultDir
		if outputDir == "" {
			outputDir = "."
		}
	}

	outPath := filepath.Join(outputDir, tf.Name)
	peerID := tf.GeneratePeerID()

	ts := &TorrentSession{
		ID:              infoHash,
		InfoHash:        infoHash,
		Name:            tf.Name,
		TotalSize:       tf.Length,
		OutputDir:       outputDir,
		OutputPath:      outPath,
		TorrentPath:     torrentPath,
		State:           StateQueued,
		NumPieces:       tf.NumPieces,
		CompletedPieces: []int{},
		tf:              tf,
		peerID:          peerID,
	}

	m.torrents[infoHash] = ts
	m.mu.Unlock()

	m.saveState()
	m.broadcastUpdate()
	return ts, nil
}

func (m *Manager) Start(id string) error {
	m.mu.Lock()
	ts, ok := m.torrents[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session: torrent not found: %s", id)
	}

	if ts.State == StateDownloading || ts.State == StateSeeding {
		m.mu.Unlock()
		return nil
	}

	ts.State = StateQueued
	m.mu.Unlock()

	go m.runTorrent(ts)
	return nil
}

func (m *Manager) Pause(id string) error {
	m.mu.Lock()
	ts, ok := m.torrents[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session: torrent not found: %s", id)
	}
	if ts.cancel != nil {
		ts.cancel()
	}
	ts.State = StatePaused
	m.mu.Unlock()
	m.saveState()
	m.broadcastUpdate()
	return nil
}

func (m *Manager) Resume(id string) error {
	return m.Start(id)
}

func (m *Manager) Remove(id string, deleteFiles bool) error {
	m.mu.Lock()
	ts, ok := m.torrents[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session: torrent not found: %s", id)
	}
	if ts.cancel != nil {
		ts.cancel()
	}
	delete(m.torrents, id)
	m.mu.Unlock()

	if deleteFiles {
		os.Remove(ts.OutputPath)
		os.Remove(ts.OutputPath + ".resume.json")
	}

	m.saveState()
	m.broadcastUpdate()
	return nil
}

func (m *Manager) runTorrent(ts *TorrentSession) {
	m.mu.Lock()
	ts.mu.Lock()
	ts.State = StateDownloading
	ts.Error = ""
	ctx, cancel := context.WithCancel(context.Background())
	ts.cancel = cancel
	ts.mu.Unlock()
	m.mu.Unlock()

	m.broadcastUpdate()

	err := m.downloadWithProgress(ctx, ts)

	ts.mu.Lock()
	if err != nil {
		if ctx.Err() != nil {
			ts.State = StatePaused
		} else {
			ts.State = StateError
			ts.Error = err.Error()
		}
	} else {
		ts.State = StateCompleted
	}
	ts.cancel = nil
	ts.mu.Unlock()

	m.saveState()
	m.broadcastUpdate()
}

func (m *Manager) downloadWithProgress(ctx context.Context, ts *TorrentSession) error {
	statePath := ts.OutputPath + ".resume.json"

	completed, err := storage.LoadResumeState(statePath)
	if err != nil {
		log.Printf("session: loading resume state: %v", err)
		completed = nil
	}
	if completed == nil {
		completed = []int{}
	}

	ts.mu.Lock()
	ts.CompletedPieces = completed
	ts.mu.Unlock()

	var outputFile *os.File
	if storage.FileExists(ts.OutputPath) && len(completed) > 0 {
		outputFile, err = storage.OpenFile(ts.OutputPath)
		if err != nil {
			return fmt.Errorf("opening output: %v", err)
		}
	} else {
		outputFile, err = storage.SaveFile(ts.OutputPath, ts.tf.Length)
		if err != nil {
			return fmt.Errorf("creating output: %v", err)
		}
	}
	defer outputFile.Close()

	ts.mu.Lock()
	ts.file = outputFile
	ts.mu.Unlock()

	pw := &progressWriter{
		w:          outputFile,
		total:      int64(len(completed)) * ts.tf.PieceLen,
		speed:      &ts.speedTracker,
		session:    ts,
	}

	peerID := ts.peerID

	var trackerResp *tracker.Response
	trackerResp, err = tracker.Announce(ts.tf, peerID, uint16(m.settings.Port))
	if err != nil {
		log.Printf("session: tracker announce: %v", err)
	}

	if trackerResp != nil {
		ts.mu.Lock()
		ts.Tracker = TrackerInfo{
			AnnounceURL:  ts.tf.Announce,
			LastAnnounce: time.Now().Format(time.RFC3339),
			NextAnnounce: time.Now().Add(time.Duration(trackerResp.Interval) * time.Second).Format(time.RFC3339),
			Seeders:      0,
			Leechers:     0,
			Status:       "ok",
		}
		ts.mu.Unlock()

		m.broadcastUpdate()
	}

	if trackerResp == nil || len(trackerResp.Peers) == 0 {
		return fmt.Errorf("session: no peers from tracker")
	}

	peers := make([]download.PeerAddr, len(trackerResp.Peers))
	for i, p := range trackerResp.Peers {
		peers[i] = download.PeerAddr{IP: p.IP, Port: p.Port}
	}

	ts.mu.Lock()
	ts.Peers = make([]PeerInfo, len(trackerResp.Peers))
	for i, p := range trackerResp.Peers {
		ts.Peers[i] = PeerInfo{
			IP:         p.IP.String(),
			Port:       int(p.Port),
			LastActive: time.Now().Format(time.RFC3339),
		}
	}
	ts.mu.Unlock()

	downloadLog := log.New(io.Discard, "", 0)

	errCh := make(chan error, 1)
	go func() {
		errCh <- download.Torrent(ts.tf, peers, peerID, pw, completed, downloadLog)
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return err
			}
			ts.mu.Lock()
			ts.Progress = 100.0
			ts.Downloaded = ts.TotalSize
			ts.mu.Unlock()
			return nil
		case <-ticker.C:
			ts.mu.Lock()
			speedBps := ts.speedTracker.Rate()
			ts.DownloadSpeed = speedBps
			pw.mu.Lock()
			downloaded := pw.total
			pw.mu.Unlock()
			ts.Downloaded = downloaded
			if ts.TotalSize > 0 {
				ts.Progress = float64(downloaded) * 100.0 / float64(ts.TotalSize)
			}
			if speedBps > 0 {
				remaining := float64(ts.TotalSize - downloaded)
				ts.ETA = remaining / speedBps
			}
			if ts.Downloaded > 0 {
				ts.Ratio = float64(ts.Uploaded) / float64(ts.Downloaded)
			}
			ts.mu.Unlock()
			m.broadcastUpdate()
		}
	}
}

func (m *Manager) broadcastUpdate() {
	m.mu.RLock()
	torrents := make([]map[string]interface{}, 0, len(m.torrents))
	for _, ts := range m.torrents {
		ts.mu.RLock()
		t := map[string]interface{}{
			"id":              ts.ID,
			"name":            ts.Name,
			"info_hash":       ts.InfoHash,
			"total_size":      ts.TotalSize,
			"downloaded":      ts.Downloaded,
			"uploaded":        ts.Uploaded,
			"state":           ts.State.String(),
			"progress":        ts.Progress,
			"download_speed":  ts.DownloadSpeed,
			"upload_speed":    ts.UploadSpeed,
			"eta":             ts.ETA,
			"ratio":           ts.Ratio,
			"num_pieces":      ts.NumPieces,
			"num_completed":   len(ts.CompletedPieces),
			"error":           ts.Error,
			"peers":           len(ts.Peers),
		}
		ts.mu.RUnlock()
		torrents = append(torrents, t)
	}
	m.mu.RUnlock()

	data, _ := json.Marshal(torrents)
	m.sse.Broadcast(string(data))
}

type progressWriter struct {
	w       io.WriteSeeker
	total   int64
	mu      sync.Mutex
	speed   *SpeedTracker
	session *TorrentSession
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if n > 0 {
		pw.mu.Lock()
		pw.total += int64(n)
		pw.mu.Unlock()
		if pw.speed != nil {
			pw.speed.Record(int64(n))
		}
	}
	return n, err
}

func (pw *progressWriter) Seek(offset int64, whence int) (int64, error) {
	return pw.w.Seek(offset, whence)
}

func DefaultSettings() Settings {
	return Settings{
		DefaultDir:      ".",
		Port:            6881,
		MaxActive:       5,
		MaxDownloadSpeed: 0,
		MaxUploadSpeed:   0,
		EnablePeers:     true,
		EnableSeeding:   false,
	}
}

func (m *Manager) saveState() {
	m.mu.RLock()
	ps := persistState{
		Settings: m.settings,
	}
	for _, ts := range m.torrents {
		ts.mu.RLock()
		ps.Torrents = append(ps.Torrents, persistTorrent{
			ID:              ts.ID,
			TorrentPath:     ts.TorrentPath,
			OutputDir:       ts.OutputDir,
			CompletedPieces: ts.CompletedPieces,
			Downloaded:      ts.Downloaded,
			Uploaded:        ts.Uploaded,
		})
		ts.mu.RUnlock()
	}
	m.mu.RUnlock()

	data, err := json.Marshal(ps)
	if err != nil {
		log.Printf("session: marshal state: %v", err)
		return
	}
	os.WriteFile("torrent-ui-state.json", data, 0644)
}

func LoadState(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m := NewManager(DefaultSettings())
			return m, nil
		}
		return nil, err
	}

	var ps persistState
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, err
	}

	m := NewManager(ps.Settings)

	for _, pt := range ps.Torrents {
		tf, err := torrent.ParseFile(pt.TorrentPath)
		if err != nil {
			log.Printf("session: loading torrent %s: %v", pt.TorrentPath, err)
			continue
		}

		infoHash := fmt.Sprintf("%x", tf.InfoHash)
		peerID := tf.GeneratePeerID()

		ts := &TorrentSession{
			ID:              infoHash,
			InfoHash:        infoHash,
			Name:            tf.Name,
			TotalSize:       tf.Length,
			OutputDir:       pt.OutputDir,
			OutputPath:      filepath.Join(pt.OutputDir, tf.Name),
			TorrentPath:     pt.TorrentPath,
			State:           StatePaused,
			NumPieces:       tf.NumPieces,
			CompletedPieces: pt.CompletedPieces,
			Downloaded:      pt.Downloaded,
			Uploaded:        pt.Uploaded,
			Progress:        float64(len(pt.CompletedPieces)) * 100.0 / float64(tf.NumPieces),
			tf:              tf,
			peerID:          peerID,
		}
		if ts.TotalSize > 0 {
			ts.Progress = float64(len(pt.CompletedPieces)) * float64(tf.PieceLen) * 100.0 / float64(ts.TotalSize)
		}

		m.torrents[infoHash] = ts
	}

	return m, nil
}

func (ts *TorrentSession) Lock() {
	ts.mu.Lock()
}

func (ts *TorrentSession) Unlock() {
	ts.mu.Unlock()
}

func (ts *TorrentSession) RLock() {
	ts.mu.RLock()
}

func (ts *TorrentSession) RUnlock() {
	ts.mu.RUnlock()
}
