package security

import (
	"fmt"
	"path/filepath"
	"strings"

	"bittorrent/internal/bencode"
	"bittorrent/internal/torrent"
)

type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}

type SuspiciousFile struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Report struct {
	RiskLevel       RiskLevel       `json:"risk_level"`
	Warnings        []string        `json:"warnings"`
	SuspiciousFiles []SuspiciousFile `json:"suspicious_files"`
	TrackerWarnings []string        `json:"tracker_warnings"`
	NetworkWarnings []string        `json:"network_warnings"`
}

var suspiciousExts = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".scr": true,
	".msi": true, ".apk": true, ".jar": true, ".vbs": true,
	".ps1": true, ".sh": true, ".dmg": true, ".reg": true,
	".com": true, ".pif": true, ".js": true, ".wsf": true,
}

var archiveExts = map[string]bool{
	".zip": true, ".rar": true, ".7z": true, ".tar": true,
	".gz": true, ".bz2": true, ".xz": true, ".zst": true,
}

func ScanTorrent(tf *torrent.FileInfo) *Report {
	report := &Report{
		Warnings:        []string{},
		SuspiciousFiles: []SuspiciousFile{},
		TrackerWarnings: []string{},
		NetworkWarnings: []string{},
	}

	report.addWarning("This scanner only performs metadata and filename checks. It cannot guarantee that downloaded content is safe.")

	if ok, ext := hasSuspiciousExtension(tf.Name); ok {
		report.SuspiciousFiles = append(report.SuspiciousFiles, SuspiciousFile{
			Name:   tf.Name,
			Reason: "suspicious file extension: " + ext,
		})
		report.addWarning("File has a potentially dangerous extension: " + ext)
	}

	if hasDoubleExtension(tf.Name) {
		report.SuspiciousFiles = append(report.SuspiciousFiles, SuspiciousFile{
			Name:   tf.Name,
			Reason: "double extension detected",
		})
		report.addWarning("File has a double extension which may hide the real file type")
	}

	if hasPathTraversal(tf.Name) {
		report.SuspiciousFiles = append(report.SuspiciousFiles, SuspiciousFile{
			Name:   tf.Name,
			Reason: "path traversal attempt",
		})
		report.addWarning("Filename contains path traversal characters")
	}

	if isHiddenFile(tf.Name) {
		report.SuspiciousFiles = append(report.SuspiciousFiles, SuspiciousFile{
			Name:   tf.Name,
			Reason: "hidden file",
		})
	}

	if isArchiveFile(tf.Name) {
		report.SuspiciousFiles = append(report.SuspiciousFiles, SuspiciousFile{
			Name:   tf.Name,
			Reason: "archive file: " + filepath.Ext(tf.Name),
		})
	}

	if isLongFileName(tf.Name) {
		report.addWarning("Filename is unusually long")
	}

	fileSize := tf.Length
	if isLargeExecutable(tf.Name, fileSize) {
		report.addWarning("Large file with suspicious extension: " + formatSize(fileSize))
	}

	if isTinyFileTorrent(tf) {
		report.addWarning("Torrent contains many small files, which could be a sign of malicious content")
	}

	checkTracker(report, tf)

	report.NetworkWarnings = append(report.NetworkWarnings,
		"This application does not perform malware scanning. Downloaded files should be scanned with trusted antivirus software.")

	report.RiskLevel = calculateRiskLevel(report)

	return report
}

func ScanRawTorrent(data []byte) *Report {
	root, err := bencode.Decode(data)
	if err != nil {
		return &Report{
			RiskLevel: RiskMedium,
			Warnings:  []string{"Could not decode torrent metadata"},
		}
	}
	dict, _ := root.AsDict()
	tf := &torrent.FileInfo{Info: dict}
	if infoVal, ok := dict["info"]; ok {
		infoDict, _ := infoVal.AsDict()
		tf.Info = infoDict
		if nameVal, ok := infoDict["name"]; ok {
			tf.Name, _ = nameVal.AsString()
		}
		if lengthVal, ok := infoDict["length"]; ok {
			tf.Length, _ = lengthVal.AsInt()
		}
		if pieceLenVal, ok := infoDict["piece length"]; ok {
			tf.PieceLen, _ = pieceLenVal.AsInt()
		}
	}
	if announceVal, ok := dict["announce"]; ok {
		tf.Announce, _ = announceVal.AsString()
	}
	return ScanTorrent(tf)
}

func (r *Report) addWarning(msg string) {
	r.Warnings = append(r.Warnings, msg)
}

func hasSuspiciousExtension(name string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(name))
	return suspiciousExts[ext], ext
}

func hasDoubleExtension(name string) bool {
	parts := strings.Split(name, ".")
	if len(parts) <= 2 {
		return false
	}
	lastExt := strings.ToLower("." + parts[len(parts)-1])
	return suspiciousExts[lastExt] || archiveExts[lastExt]
}

func hasPathTraversal(name string) bool {
	return strings.Contains(name, "../") || strings.Contains(name, "..\\") ||
		strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\")
}

func isHiddenFile(name string) bool {
	return strings.HasPrefix(name, ".")
}

func isArchiveFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return archiveExts[ext]
}

func isLongFileName(name string) bool {
	return len(name) > 200
}

func isLargeExecutable(name string, size int64) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if !suspiciousExts[ext] {
		return false
	}
	return size > 50*1024*1024
}

func isTinyFileTorrent(tf *torrent.FileInfo) bool {
	if tf.PieceLen > 0 && tf.PieceLen < 16384 {
		numPieces := int64(tf.NumPieces)
		return numPieces > 100
	}
	return false
}

func checkTracker(report *Report, tf *torrent.FileInfo) {
	if strings.HasPrefix(tf.Announce, "http://") {
		report.TrackerWarnings = append(report.TrackerWarnings,
			"Tracker uses plain HTTP instead of HTTPS: "+tf.Announce)
		report.addWarning("Tracker connection is not encrypted (HTTP, not HTTPS)")
	}
}

func calculateRiskLevel(r *Report) RiskLevel {
	riskScore := 0

	for _, f := range r.SuspiciousFiles {
		if strings.Contains(f.Reason, "suspicious file extension") {
			riskScore += 3
		}
		if strings.Contains(f.Reason, "double extension") {
			riskScore += 4
		}
		if strings.Contains(f.Reason, "path traversal") {
			riskScore += 5
		}
	}

	riskScore += len(r.TrackerWarnings) * 2

	if riskScore >= 5 {
		return RiskHigh
	}
	if riskScore >= 2 {
		return RiskMedium
	}
	return RiskLow
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

func (r *Report) RiskLabel() string {
	return strings.ToUpper(r.RiskLevel.String())
}
