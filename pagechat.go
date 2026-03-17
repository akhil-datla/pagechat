package pagechat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Config configures a PageChat server.
type Config struct {
	// Addr is the TCP address to listen on (default ":8080").
	Addr string

	// MessageTTL is how long messages are kept in history (default 24h).
	MessageTTL time.Duration

	// MaxMessages is the maximum messages stored per room (default 1000).
	MaxMessages int

	// ContentFilter enables profanity filtering on messages (default false).
	ContentFilter bool
}

// Server is a PageChat WebSocket chat server.
type Server struct {
	hub       *hub
	config    Config
	handler   http.Handler
	server    *http.Server
	upgrader  websocket.Upgrader
	startTime time.Time
}

// NewServer creates a new PageChat server with the given configuration.
func NewServer(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.MessageTTL == 0 {
		cfg.MessageTTL = 24 * time.Hour
	}
	if cfg.MaxMessages == 0 {
		cfg.MaxMessages = 1000
	}

	s := &Server{
		hub:    newHub(cfg.MessageTTL, cfg.MaxMessages, cfg.ContentFilter),
		config: cfg,
	}

	s.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	mux.HandleFunc("GET /api/messages", s.handleMessages)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.Handle("/", webHandler())

	s.handler = s.withCORS(mux)
	s.server = &http.Server{
		Addr:              cfg.Addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// Handler returns the HTTP handler, useful for testing or embedding.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Start starts the server and blocks until it's shut down.
func (s *Server) Start() error {
	s.startTime = time.Now()
	s.hub.startCleanup()
	slog.Info("pagechat server started", "addr", s.config.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.stop()
	return s.server.Shutdown(ctx)
}

// Close stops the hub cleanup goroutine without shutting down the HTTP server.
// Useful in tests where httptest.Server manages the listener.
func (s *Server) Close() {
	s.hub.stop()
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	website := r.URL.Query().Get("website")
	if website == "" {
		http.Error(w, `{"error":"missing website parameter"}`, http.StatusBadRequest)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	c := &client{
		hub:     s.hub,
		conn:    conn,
		send:    make(chan Message, sendBufSize),
		website: website,
	}

	s.hub.register(c)

	go c.writePump()
	go c.readPump()
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	website := r.URL.Query().Get("website")
	if website == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing website parameter"})
		return
	}

	msgs := s.hub.getMessages(website)
	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hub.stats(s.startTime))
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
