# EO WebSocket Proxy

WebSocket proxy for Endless Online servers, written in Go.

## Setup

1. Clone and build:
   ```bash
   git clone https://github.com/tabbyed/eo-websocket-proxy.git
   cd eo-websocket-proxy
   go mod download
   go build .
   ```

2. Run:
   ```bash
   ./eo-websocket-proxy
   ```

3. Open control panel: `http://localhost:8080`

## Usage

1. Set WebSocket port (default: 8080)
2. Enter EO server address (e.g., `gameserver.ddns.eo-rs.dev:8078`)
3. Click "Start Bridge"
4. Connect web clients to `ws://localhost:[port]`

## API

- `GET /api/status` - Bridge status
- `POST /api/start` - Start bridge with `{"port": 8080, "server": "host:port"}`
- `POST /api/stop` - Stop bridge
- `GET /api/servers` - List discovered servers

## Files

- `main.go` - Proxy server
- `static/` - Web control panel