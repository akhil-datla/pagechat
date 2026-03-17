# PageChat

**Real-time chat for any website.** Drop a script tag on your site and let visitors chat with each other — no accounts, no databases, no third-party services.

[![CI](https://github.com/akhil-datla/pagechat/actions/workflows/ci.yml/badge.svg)](https://github.com/akhil-datla/pagechat/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/akhil-datla/pagechat)](https://goreportcard.com/report/github.com/akhil-datla/pagechat)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

## Features

- **Per-page chat rooms** — visitors on the same URL automatically share a room
- **Built-in UI** — beautiful chat interface at `/` and an embeddable widget for any site
- **Zero configuration** — single binary, runs anywhere
- **Profanity filter** — built-in content filtering powered by [go-away](https://github.com/TwiN/go-away)
- **Message history** — configurable TTL, served over a REST API
- **Lightweight** — no database required, messages stored in-memory
- **Embeddable** — use as a standalone server or import as a Go library
- **Production-ready** — graceful shutdown, structured logging, health checks, CORS

## Quick Start

### Binary

```bash
# Install
go install github.com/akhil-datla/pagechat/cmd/pagechat@latest

# Run
pagechat --port 8080 --filter
```

### Docker

```bash
docker run -p 8080:8080 ghcr.io/akhil-datla/pagechat:latest
```

### From Source

```bash
git clone https://github.com/akhil-datla/pagechat.git
cd pagechat
make run
```

## Built-in UI

Start the server and open `http://localhost:8080` in your browser to use the full-page chat interface. It includes username selection, message history, connection status, and auto-reconnect.

## Embeddable Widget

Add a single script tag to any website to get a floating chat bubble:

```html
<script src="https://your-pagechat-server.com/widget.js"></script>
```

The widget automatically detects the server URL from the script's `src` attribute.

### Widget Options

Configure via `data-*` attributes on the script tag:

```html
<script
  src="https://your-pagechat-server.com/widget.js"
  data-position="bottom-left"
  data-theme="dark"
></script>
```

| Attribute | Default | Options |
|---|---|---|
| `data-server` | *(auto-detected)* | Override server URL |
| `data-position` | `bottom-right` | `bottom-right`, `bottom-left` |
| `data-theme` | `light` | `light`, `dark` |

## Custom Integration

For full control, connect directly via WebSocket:

```html
<script>
  const ws = new WebSocket(
    `ws://localhost:8080/ws?website=${encodeURIComponent(location.href)}`
  );

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log(`[${msg.username}] ${msg.content}`);
  };

  function sendMessage(username, content) {
    ws.send(JSON.stringify({ username, content }));
  }
</script>
```

### Fetch Message History

```javascript
const url = encodeURIComponent(location.href);
const res = await fetch(`http://localhost:8080/api/messages?website=${url}`);
const messages = await res.json();
```

## API Reference

| Endpoint | Method | Description |
|---|---|---|
| `/` | `GET` | Built-in chat UI |
| `/widget.js` | `GET` | Embeddable chat widget script |
| `/ws?website=<url>` | `GET` | WebSocket connection — joins the room for `<url>` |
| `/api/messages?website=<url>` | `GET` | Returns message history as JSON array |
| `/api/health` | `GET` | Health check — returns `{"status": "ok"}` |
| `/api/stats` | `GET` | Server stats — rooms, clients, messages, uptime |

### Message Format

**Send** (client → server):
```json
{ "username": "alice", "content": "Hello everyone!" }
```

**Receive** (server → client):
```json
{
  "website": "https://example.com/page",
  "username": "alice",
  "content": "Hello everyone!",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Configuration

| Flag | Default | Description |
|---|---|---|
| `--port` | `8080` | Server port |
| `--ttl` | `24h` | Message time-to-live |
| `--max-messages` | `1000` | Max messages stored per room |
| `--filter` | `true` | Enable profanity filter |
| `--version` | | Print version and exit |

## Use as a Library

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/akhil-datla/pagechat"
)

func main() {
    srv := pagechat.NewServer(pagechat.Config{
        Addr:          ":9090",
        MessageTTL:    12 * time.Hour,
        MaxMessages:   500,
        ContentFilter: true,
    })

    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Architecture

```
┌─────────┐   WebSocket   ┌──────────┐   broadcast   ┌──────┐
│ Client 1 │──────────────▶│          │──────────────▶│Room A│
└─────────┘               │          │               └──────┘
                          │  Server  │
┌─────────┐   WebSocket   │          │   broadcast   ┌──────┐
│ Client 2 │──────────────▶│          │──────────────▶│Room B│
└─────────┘               │   Hub    │               └──────┘
                          │          │
┌─────────┐   REST API    │          │
│ Client 3 │──────────────▶│          │
└─────────┘               └──────────┘
```

- **Server** — HTTP server with graceful shutdown, CORS, and routing
- **Hub** — manages rooms, broadcasts messages, stores history
- **Client** — WebSocket connection with read/write pumps and keepalive
- **Room** — per-website group of connected clients

## Development

```bash
make test       # Run tests with race detector
make cover      # Generate coverage report
make lint       # Run golangci-lint
make build      # Build binary to bin/
make help       # Show all targets
```

## License

[GNU General Public License v3.0](LICENSE)

---

Built by [Akhil Datla](https://github.com/akhil-datla)
