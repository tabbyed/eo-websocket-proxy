package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ethanmoffat/eolib-go/v3/data"
	"github.com/gorilla/websocket"
)

// Server - if API call then parse this JSON structure
type Server struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Version    string `json:"version"`
	Zone       string `json:"zone"`
	Players    int    `json:"players"`
	Site       string `json:"site"`
	ClientSite string `json:"clientsite"`
}

// Bridge - if browser connects then route to game server
type Bridge struct {
	webPort    string
	gameServer string
	server     *http.Server
	upgrader   websocket.Upgrader
	mu         sync.Mutex
	running    bool
}

// App - if control panel then manage bridge and servers
type App struct {
	bridge      *Bridge
	controlPort int
	servers     []Server
	mu          sync.RWMutex
}

// NewBridge - if create bridge then set host:port and game target
func NewBridge(host string, port int, gameServer string) *Bridge {
	return &Bridge{
		webPort:    fmt.Sprintf("%s:%d", host, port),
		gameServer: gameServer,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true }, // if origin check then allow all
		},
	}
}

// Start - if start bridge then listen for browser connections
func (b *Bridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("already started") // if already running then error
	}

	b.server = &http.Server{
		Addr:    b.webPort,
		Handler: http.HandlerFunc(b.handleConnection),
	}
	b.running = true

	// if start then run server in background
	go func() {
		b.server.ListenAndServe()
	}()

	return nil
}

// Stop - if stop bridge then close server
func (b *Bridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil // if not running then nothing to do
	}

	b.running = false
	if b.server != nil {
		return b.server.Close() // if server exists then close it
	}
	return nil
}

// IsRunning - if check status then return running state
func (b *Bridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// handleConnection - if browser connects then bridge to game server
func (b *Bridge) handleConnection(w http.ResponseWriter, r *http.Request) {
	// if websocket upgrade then create connection
	webConn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer webConn.Close()

	// if game server connect then create TCP connection
	gameConn, err := net.Dial("tcp", b.gameServer)
	if err != nil {
		return
	}
	defer gameConn.Close()

	log.Printf("Bridge active: %s", r.RemoteAddr)

	// if connections ready then start forwarding both ways
	done := make(chan bool, 2)
	go b.forwardWebToGame(webConn, gameConn, done) // browser -> game
	go b.forwardGameToWeb(webConn, gameConn, done) // game -> browser

	<-done // if either direction fails then close connection
}

// forwardWebToGame - if browser sends data then forward to game
func (b *Bridge) forwardWebToGame(webConn *websocket.Conn, gameConn net.Conn, done chan bool) {
	defer func() { done <- true }() // if function exits then signal done

	for {
		msgType, data, err := webConn.ReadMessage()
		if err != nil {
			return // if read error then close connection
		}
		if msgType == websocket.BinaryMessage {
			gameConn.Write(data) // if binary message then send to game
		}
	}
}

// forwardGameToWeb - if game sends packet then forward to browser
func (b *Bridge) forwardGameToWeb(webConn *websocket.Conn, gameConn net.Conn, done chan bool) {
	defer func() { done <- true }() // if function exits then signal done

	for {
		packet, err := b.readGamePacket(gameConn)
		if err != nil {
			return // if read error then close connection
		}
		if err := webConn.WriteMessage(websocket.BinaryMessage, packet); err != nil {
			return // if write error then close connection
		}
	}
}

// readGamePacket - if game data then read complete EO packet
func (b *Bridge) readGamePacket(gameConn net.Conn) ([]byte, error) {
	// if packet starts then read 2-byte length header
	lengthBytes := make([]byte, 2)
	if _, err := io.ReadFull(gameConn, lengthBytes); err != nil {
		return nil, err
	}

	// if length known then read packet body
	packetLength := int(data.DecodeNumber(lengthBytes))
	packetData := make([]byte, packetLength)
	if _, err := io.ReadFull(gameConn, packetData); err != nil {
		return nil, err
	}

	// if complete then return header + body
	return append(lengthBytes, packetData...), nil
}

// fetchServers - if refresh servers then get from API
func (a *App) fetchServers() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://apollo-games.com/SLN/sln.php/api")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var servers []Server
	if err := json.Unmarshal(body, &servers); err != nil {
		return err
	}

	// if parse success then update server list
	a.mu.Lock()
	a.servers = servers
	a.mu.Unlock()

	return nil
}

// handleAPIStatus - if status request then return bridge state
func (a *App) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	running := a.bridge != nil && a.bridge.IsRunning()
	status := "Bridge Stopped"
	port := ""

	if running {
		port = a.bridge.webPort[2:]
		status = fmt.Sprintf("Bridge Running - ws://localhost:%s", port)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"running": running,
		"status":  status,
		"port":    port,
	})
}

// handleAPIServers - if servers request then return server list
func (a *App) handleAPIServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	a.mu.RLock()
	servers := a.servers
	a.mu.RUnlock()

	json.NewEncoder(w).Encode(servers)
}

// handleAPIStart - if start request then create and start bridge
func (a *App) handleAPIStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Port   int    `json:"port"`
		Server string `json:"server"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid request"})
		return
	}

	if a.bridge != nil && a.bridge.IsRunning() {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Bridge already running"})
		return
	}

	// if valid request then create and start bridge
	a.bridge = NewBridge("0.0.0.0", req.Port, req.Server)
	if err := a.bridge.Start(); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleAPIStop - if stop request then shutdown bridge
func (a *App) handleAPIStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if a.bridge != nil {
		a.bridge.Stop() // if bridge exists then stop it
		a.bridge = nil  // if stopped then clear reference
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleAPIRefreshServers - if refresh request then fetch new server list
func (a *App) handleAPIRefreshServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := a.fetchServers(); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func main() {
	app := &App{controlPort: 8081}

	// Attempt to load available servers at startup
	app.fetchServers()

	// Serve static assets (CSS, JS, etc.) from ./static/
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// API endpoints
	http.HandleFunc("/api/status", app.handleAPIStatus)
	http.HandleFunc("/api/servers", app.handleAPIServers)
	http.HandleFunc("/api/start", app.handleAPIStart)
	http.HandleFunc("/api/stop", app.handleAPIStop)
	http.HandleFunc("/api/refresh-servers", app.handleAPIRefreshServers)

	// Serve index.html for root path only
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "./static/index.html")
	})

	// Log and start control server
	addr := fmt.Sprintf(":%d", app.controlPort)
	log.Printf("Starting control panel on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Control panel failed: %v", err)
	}
}
