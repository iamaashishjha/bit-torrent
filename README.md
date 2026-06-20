# Build Your Own BitTorrent Client

A minimal BitTorrent client written in Go from scratch. No full-stack BitTorrent libraries — just the standard library and a deep understanding of the protocol.

## What is BitTorrent?

BitTorrent is a peer-to-peer (P2P) file sharing protocol. Instead of downloading a file from a single server, you download pieces of the file from multiple peers simultaneously. Each piece is verified using SHA1 hashes before being written to disk.

Key components:
- **Torrent file** — a metadata file describing the file(s) to download.
- **Tracker** — an HTTP server that coordinates peers.
- **Peers** — other clients sharing the same file.
- **Pieces** — the file is split into fixed-size pieces, each identified by a SHA1 hash.

## What is Bencode?

**Bencode** ("Ben-code") is the encoding format used by BitTorrent for metadata files and tracker communication. It supports four types:

| Type       | Encoding              | Example                 |
|------------|-----------------------|-------------------------|
| String     | `<length>:<data>`     | `4:spam` → `"spam"`    |
| Integer    | `i<number>e`          | `i42e` → `42`          |
| List       | `l<values>e`          | `l4:spami42ee` → `["spam", 42]` |
| Dictionary | `d<key-value pairs>e` | `d3:bar4:spame` → `{"bar": "spam"}`  |

Dictionary keys must be strings and are stored in sorted order.

## What a Torrent File Contains

A `.torrent` file is a bencoded dictionary with these keys:

| Key         | Description |
|-------------|-------------|
| `announce`  | The tracker URL |
| `info`      | A dictionary describing the file(s) |
| `name`      | Suggested file name |
| `length`    | File size in bytes |
| `piece length` | Size of each piece in bytes |
| `pieces`    | Concatenated 20-byte SHA1 hashes of each piece |

The **info hash** is the SHA1 hash of the bencoded `info` dictionary. This uniquely identifies the torrent and is used in tracker requests and peer handshakes.

## How Tracker Communication Works

The client sends an HTTP GET request to the announce URL with these query parameters:

| Parameter    | Description |
|-------------|-------------|
| `info_hash` | 20-byte SHA1 of the info dict |
| `peer_id`   | 20-byte unique client ID |
| `port`      | Listening port (default 6881) |
| `uploaded`  | Bytes uploaded so far |
| `downloaded`| Bytes downloaded so far |
| `left`      | Bytes remaining |
| `compact`   | `1` for compact peer representation |

The tracker responds with a bencoded dictionary containing:
- `interval` — seconds to wait between re-announces
- `peers` — compact peer list (6 bytes per peer: 4 bytes IP + 2 bytes port)

## How Peer Handshake Works

After connecting to a peer over TCP, the client sends a 68-byte handshake:

```
Offset  Size  Description
0       1     Protocol length (always 19)
1       19    Protocol string ("BitTorrent protocol")
20      8     Reserved bytes (all zeros)
28      20    Info hash
48      20    Peer ID
```

The peer responds with the same structure. If the info hash matches, the connection is established.

## How Pieces and Blocks Are Downloaded

1. **Connect** to a peer via TCP
2. **Handshake** — exchange protocol info and info hash
3. **Wait for bitfield** — the peer advertises which pieces it has
4. **Send interested** — tell the peer you want data
5. **Wait for unchoke** — wait until the peer allows you to download
6. **Request blocks** — each piece is requested in 16 KiB blocks
7. **Receive piece data** — assemble blocks back into a full piece
8. **Verify SHA1** — check the piece matches its expected hash
9. **Write to disk** — seek to the correct offset and write the piece

Messages on the wire use this format:
```
Length (4 bytes) | Message ID (1 byte) | Payload (variable)
```

Message IDs:
- `0` choke, `1` unchoke, `2` interested, `3` not interested
- `4` have, `5` bitfield, `6` request, `7` piece, `8` cancel

## Project Structure

```
.
├── cmd/
│   ├── torrent/main.go          # CLI entry point (headless)
│   └── torrent-ui/main.go       # Web UI entry point
├── internal/
│   ├── bencode/                 # Bencode decoder/encoder
│   ├── torrent/                 # Torrent metadata parser
│   ├── tracker/                 # HTTP tracker protocol
│   ├── peer/                    # Peer wire protocol
│   ├── download/                # Download orchestration
│   ├── storage/                 # File I/O and resume state
│   ├── session/                 # Session manager (multi-torrent)
│   ├── security/                # Security scanner (filename checks)
│   └── api/                     # REST API + SSE + template rendering
├── web/
│   ├── templates/               # Go HTML templates (7 pages)
│   └── static/                  # CSS + HTMX library
├── scripts/
│   └── gen-sample-torrent.go    # Generate a sample .torrent
├── sample.torrent
├── go.mod
└── README.md
```

## How to Run

### CLI Mode (headless)

```bash
go run ./cmd/torrent --torrent ./sample.torrent --out ./downloads
```

| Flag       | Default | Description |
|------------|---------|-------------|
| `--torrent` | —      | Path to .torrent file (required) |
| `--out`     | `.`    | Output directory |
| `--port`    | `6881` | Listening port for tracker announces |

### UI Mode

```bash
go run ./cmd/torrent-ui
# → open http://localhost:8080
```

Set the port via the `PORT` environment variable:
```bash
PORT=9090 go run ./cmd/torrent-ui
```

### Both Modes

```bash
# Generate a sample torrent
go run ./scripts/gen-sample-torrent.go sample.torrent my-content.dat

# Run tests
go test ./... -v
```

## Running Tests

```bash
go test ./... -v
```

## UI Features

### Dashboard
- List all torrents with name, size, status, progress bar, speeds, ETA, ratio
- Live updates every 2 seconds via HTMX polling + SSE
- Start, pause, resume, and remove torrents from the dashboard

### Add Torrent
- Upload `.torrent` files via browser
- Choose download directory
- Torrent is validated before adding
- Auto-starts download after adding

### Torrent Detail
- Full metadata view (info hash, size, downloaded, speeds, ratio)
- Large progress bar with piece completion count
- Security risk summary
- Quick actions (pause, resume, remove)
- Tab navigation to Peers / Trackers / Security pages

### Peers Page
- Shows all discovered peers from tracker
- IP address, port, last active time

### Trackers Page
- Announce URL
- Status, last announce time, next announce time

### Security Report
- Overall risk level: Low / Medium / High
- Warnings list
- Suspicious files table with reasons
- Tracker warnings (HTTP vs HTTPS)
- Network warnings and disclaimer
- See Security Scanner section below for full details

### Settings
- Default download directory
- Listen port
- Max active downloads
- Max download speed
- Max upload speed

## REST API

All endpoints return JSON. The UI uses these via HTMX for live updates.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/torrents` | List all torrents |
| `POST` | `/api/torrents` | Add torrent (multipart form) |
| `GET` | `/api/torrents/{id}` | Get torrent details |
| `POST` | `/api/torrents/{id}/start` | Start download |
| `POST` | `/api/torrents/{id}/pause` | Pause download |
| `POST` | `/api/torrents/{id}/resume` | Resume download |
| `DELETE` | `/api/torrents/{id}` | Remove torrent |
| `DELETE` | `/api/torrents/{id}?delete_files=true` | Remove torrent + files |
| `GET` | `/api/torrents/{id}/peers` | List peers for a torrent |
| `GET` | `/api/torrents/{id}/trackers` | Get tracker info |
| `GET` | `/api/torrents/{id}/security-report` | Get security scan report |
| `GET` | `/api/events` | SSE stream of torrent updates |
| `GET` | `/api/settings` | Get current settings |
| `POST` | `/api/settings` | Update settings |

## Security Scanner

The security scanner performs metadata-only checks on torrent files. It **does not** scan the actual downloaded content.

### Checks Performed

- **Suspicious file extensions**: `.exe`, `.bat`, `.cmd`, `.scr`, `.msi`, `.apk`, `.jar`, `.vbs`, `.ps1`, `.sh`, `.dmg`, `.reg`, `.com`, `.pif`, `.js`, `.wsf`
- **Double extensions**: Files like `movie.mp4.exe` or `document.pdf.scr`
- **Path traversal**: Filenames containing `../` or absolute paths
- **Hidden files**: Files starting with `.`
- **Archive files**: `.zip`, `.rar`, `.7z`, `.tar`, `.gz`, `.bz2`, `.xz`
- **Long filenames**: Names over 200 characters
- **Large executable files**: Suspicious extensions over 50 MB
- **Many tiny files**: Large number of pieces under 16 KB
- **HTTP trackers**: Warnings for non-HTTPS tracker URLs

### Risk Scoring

| Factor | Points |
|--------|--------|
| Suspicious extension | +3 |
| Double extension | +4 |
| Path traversal | +5 |
| HTTP tracker (not HTTPS) | +2 |

- **Low** (0-1): No significant risks detected
- **Medium** (2-4): Some suspicious indicators
- **High** (5+): Multiple strong risk indicators

### Disclaimer

The security scanner performs metadata and filename checks only. It cannot guarantee that downloaded content is safe. Always scan downloaded files with trusted antivirus software before opening them.

## Implementation Details

### Session Manager (`internal/session/`)

Manages multiple torrent downloads in parallel goroutines. Each torrent runs the existing `download.Torrent()` function with a progress-wrapping writer. A speed tracker samples bytes downloaded over a 10-second window.

### Persistence

- Session state saved to `torrent-ui-state.json` (JSON)
- On restart, all previous torrents are loaded in "paused" state
- Each torrent's resume data is stored alongside the downloaded file
- Torrent files added via UI are copied to the output directory

### Live Updates

Server-Sent Events at `/api/events` broadcast torrent state as JSON every 1 second during active downloads. The UI's dashboard polls `/api/torrents` every 2 seconds via HTMX `hx-trigger="every 2s"`.

### Frontend Stack

- **Go `html/template`** for server-rendered HTML
- **HTMX** for AJAX interactions (form submission, polling, dynamic updates)
- **Plain CSS** with a dark theme inspired by GitHub's design

## Limitations

- Single-file torrents only (no multi-file support)
- HTTP/HTTPS trackers only (no UDP trackers)
- Sequential piece download from a single peer
- No DHT, PEX, or magnet links
- No encryption (plain protocol only)
- No download/upload rate limiting in the core
- No seeding support
- Security scanner only checks metadata, not actual file content

## Future Improvements

- [ ] Multi-file torrent support
- [ ] UDP tracker protocol (BEP 15)
- [ ] Concurrent downloads from multiple peers
- [ ] Piece selection strategies (rarest-first, endgame mode)
- [ ] DHT (BEP 5) for trackerless torrents
- [ ] PEX (peer exchange)
- [ ] Magnet links
- [ ] Protocol encryption (BEP 10)
- [ ] Download/upload bandwidth throttling
- [ ] Seeding after download completes
- [ ] Web UI or TUI for monitoring
- [ ] IP blocklist support
- [ ] Real-time peer connection state in UI
- [ ] Bandwidth usage graphs
- [ ] Docker support
