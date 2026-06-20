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
├── cmd/torrent/main.go          # CLI entry point
├── internal/
│   ├── bencode/                 # Bencode decoder/encoder
│   │   ├── types.go             # Value type and Encode()
│   │   ├── decode.go            # Recursive descent parser
│   │   └── decode_test.go       # Parser tests
│   ├── torrent/                 # Torrent metadata parser
│   │   ├── torrent.go           # Parse .torrent files, info_hash
│   │   └── torrent_test.go      # Metadata tests
│   ├── tracker/                 # HTTP tracker protocol
│   │   ├── tracker.go           # Announce, compact peer parsing
│   │   └── tracker_test.go      # Tracker tests
│   ├── peer/                    # Peer wire protocol
│   │   ├── handshake.go         # BitTorrent handshake
│   │   ├── handshake_test.go    # Handshake encode/decode tests
│   │   ├── message.go           # Peer messages (request, piece, etc.)
│   │   └── message_test.go      # Message serialize/parse tests
│   ├── download/                # Download orchestration
│   │   └── download.go          # Piece download, block assembly, hash verify
│   └── storage/                 # File I/O and resume state
│       └── storage.go           # File creation, resume state JSON
├── scripts/
│   └── gen-sample-torrent.go    # Generate a sample .torrent for testing
├── sample.torrent               # Example torrent file
├── go.mod
└── README.md
```

## How to Run

```bash
# Build the client
go build -o torrent ./cmd/torrent

# Run with a torrent file
./torrent --torrent ./sample.torrent --out ./downloads

# Specify a custom port
./torrent --torrent ./sample.torrent --out ./downloads --port 6881

# Generate a sample torrent from a content file
go run ./scripts/gen-sample-torrent.go my.torrent my-content.dat
```

### Flags

| Flag       | Default | Description |
|------------|---------|-------------|
| `--torrent` | —      | Path to .torrent file (required) |
| `--out`     | `.`    | Output directory |
| `--port`    | `6881` | Listening port for tracker announces |

### Output

The client logs:
- Tracker URL and response
- Number of peers discovered
- Connection attempts
- Handshake success/failure
- Per-piece download progress
- SHA1 verification results
- Download completion

Resume state is saved to `<output-file>.resume.json`.

## Running Tests

```bash
go test ./... -v
```

## Limitations

- Single-file torrents only (no multi-file support)
- HTTP/HTTPS trackers only (no UDP trackers)
- Sequential piece download from a single peer
- No DHT, PEX, or magnet links
- No encryption (plain protocol only)
- No download/upload rate limiting
- No seeding support

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
