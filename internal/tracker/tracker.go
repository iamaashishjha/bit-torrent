package tracker

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"bittorrent/internal/bencode"
	"bittorrent/internal/torrent"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

type Response struct {
	Interval int
	Peers    []Peer
}

func Announce(tf *torrent.FileInfo, peerID [20]byte, port uint16) (*Response, error) {
	base, err := url.Parse(tf.Announce)
	if err != nil {
		return nil, fmt.Errorf("tracker: parsing announce URL: %v", err)
	}

	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("tracker: unsupported scheme %q (only HTTP/HTTPS supported)", base.Scheme)
	}

	params := url.Values{}
	params.Add("info_hash", string(tf.InfoHash[:]))
	params.Add("peer_id", string(peerID[:]))
	params.Add("port", strconv.Itoa(int(port)))
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", strconv.FormatInt(tf.Length, 10))
	params.Add("compact", "1")

	base.RawQuery = params.Encode()
	reqURL := base.String()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("tracker: HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tracker: reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return parseResponse(body)
}

func parseResponse(body []byte) (*Response, error) {
	root, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("tracker: parsing bencode response: %v", err)
	}

	dict, err := root.AsDict()
	if err != nil {
		return nil, fmt.Errorf("tracker: response is not a dict: %v", err)
	}

	if failure, ok := dict["failure reason"]; ok {
		reason, _ := failure.AsString()
		return nil, fmt.Errorf("tracker: %s", reason)
	}

	tr := &Response{}

	if intervalVal, ok := dict["interval"]; ok {
		interval, err := intervalVal.AsInt()
		if err != nil {
			return nil, fmt.Errorf("tracker: invalid interval: %v", err)
		}
		tr.Interval = int(interval)
	}

	peersVal, ok := dict["peers"]
	if !ok {
		return nil, fmt.Errorf("tracker: missing 'peers' in response")
	}

	if peersVal.Type == bencode.String {
		tr.Peers, err = parseCompactPeers([]byte(peersVal.Str))
		if err != nil {
			return nil, fmt.Errorf("tracker: parsing compact peers: %v", err)
		}
	} else if peersVal.Type == bencode.List {
		return nil, fmt.Errorf("tracker: non-compact peer lists not supported")
	} else {
		return nil, fmt.Errorf("tracker: 'peers' has unexpected type")
	}

	return tr, nil
}

func parseCompactPeers(data []byte) ([]Peer, error) {
	if len(data)%6 != 0 {
		return nil, fmt.Errorf("compact peer data length %d is not a multiple of 6", len(data))
	}

	peers := make([]Peer, 0, len(data)/6)
	for i := 0; i < len(data); i += 6 {
		ip := net.IP(data[i : i+4])
		port := binary.BigEndian.Uint16(data[i+4 : i+6])
		peers = append(peers, Peer{IP: ip, Port: port})
	}

	return peers, nil
}
