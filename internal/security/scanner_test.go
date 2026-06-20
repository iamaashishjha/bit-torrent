package security

import (
	"strings"
	"testing"

	"bittorrent/internal/torrent"
)

func makeTorrentInfo(name string, length int64, announce string) *torrent.FileInfo {
	return &torrent.FileInfo{
		Name:     name,
		Length:   length,
		Announce: announce,
		NumPieces: 1,
		PieceLen:  length,
	}
}

func TestScanSafeFile(t *testing.T) {
	tf := makeTorrentInfo("ubuntu-24.04-desktop.iso", 5*1024*1024*1024, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel == RiskHigh {
		t.Fatalf("expected low risk for ISO, got %s", r.RiskLevel)
	}
}

func TestScanExeFile(t *testing.T) {
	tf := makeTorrentInfo("crack.exe", 10*1024*1024, "http://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk for .exe with HTTP tracker, got %s", r.RiskLevel)
	}
	if len(r.SuspiciousFiles) == 0 {
		t.Fatal("expected suspicious file for .exe")
	}
}

func TestScanDoubleExtension(t *testing.T) {
	tf := makeTorrentInfo("movie.mp4.exe", 100*1024*1024, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk for double extension, got %s", r.RiskLevel)
	}
	found := false
	for _, f := range r.SuspiciousFiles {
		if f.Name == "movie.mp4.exe" && f.Reason == "double extension detected" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected double extension warning")
	}
}

func TestScanPathTraversal(t *testing.T) {
	tf := makeTorrentInfo("../../etc/passwd", 100, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk for path traversal, got %s", r.RiskLevel)
	}
}

func TestScanHiddenFile(t *testing.T) {
	tf := makeTorrentInfo(".secret.txt", 100, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	found := false
	for _, f := range r.SuspiciousFiles {
		if f.Reason == "hidden file" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected hidden file warning")
	}
}

func TestScanHTTPTracker(t *testing.T) {
	tf := makeTorrentInfo("file.txt", 100, "http://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if len(r.TrackerWarnings) == 0 {
		t.Fatal("expected tracker warning for HTTP")
	}
}

func TestScanLargeExecutable(t *testing.T) {
	tf := makeTorrentInfo("setup.exe", 200*1024*1024, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	found := false
	for _, w := range r.Warnings {
		if len(w) > 0 {
			found = true
		}
	}
	if found == false {
		t.Fatal("expected at least one warning for large exe")
	}
}

func TestRiskLow(t *testing.T) {
	tf := makeTorrentInfo("readme.txt", 100, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel != RiskLow {
		t.Fatalf("expected low risk, got %s", r.RiskLevel)
	}
}

func TestRiskMedium(t *testing.T) {
	tf := makeTorrentInfo("file.bat", 100, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel != RiskMedium {
		t.Fatalf("expected medium risk for .bat with HTTPS, got %s", r.RiskLevel)
	}
}

func TestRiskHigh(t *testing.T) {
	tf := makeTorrentInfo("file.bat", 100, "http://tracker.example.com/announce")
	r := ScanTorrent(tf)
	if r.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk for .bat with HTTP tracker, got %s", r.RiskLevel)
	}
}

func TestSuspiciousExtensions(t *testing.T) {
	exts := []string{".exe", ".bat", ".cmd", ".scr", ".msi", ".apk", ".jar", ".vbs", ".ps1", ".sh"}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			tf := makeTorrentInfo("file"+ext, 100, "https://tracker.example.com/announce")
			r := ScanTorrent(tf)
			found := false
			for _, f := range r.SuspiciousFiles {
				if f.Name == "file"+ext {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected suspicious file for extension %s", ext)
			}
		})
	}
}

func TestArchiveExtensions(t *testing.T) {
	exts := []string{".zip", ".rar", ".7z", ".tar", ".gz"}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			tf := makeTorrentInfo("archive"+ext, 100, "https://tracker.example.com/announce")
			r := ScanTorrent(tf)
			found := false
			for _, f := range r.SuspiciousFiles {
				if strings.Contains(f.Reason, "archive") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected archive warning for extension %s", ext)
			}
		})
	}
}

func TestScanRawTorrent(t *testing.T) {
	data := []byte("d8:announce35:http://tracker.example.com/announce4:infod4:name8:evil.exe6:lengthi100e12:piece lengthi100e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	r := ScanRawTorrent(data)
	if r.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk for .exe in raw torrent, got %s", r.RiskLevel)
	}
}

func TestLongFileName(t *testing.T) {
	longName := strings.Repeat("a", 250) + ".txt"
	tf := makeTorrentInfo(longName, 100, "https://tracker.example.com/announce")
	r := ScanTorrent(tf)
	found := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "unusually long") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected warning for long filename")
	}
}
