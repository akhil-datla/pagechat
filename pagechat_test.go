package pagechat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	srv := NewServer(Config{
		MessageTTL:    time.Hour,
		MaxMessages:   100,
		ContentFilter: false,
	})
	srv.startTime = time.Now()
	srv.hub.startCleanup()
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		srv.Close()
		ts.Close()
	})
	return srv, ts
}

func wsConnect(t *testing.T, ts *httptest.Server, website string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?website=" + website
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestWebSocketConnection(t *testing.T) {
	_, ts := newTestServer(t)

	conn := wsConnect(t, ts, "example.com")

	// Send a message.
	msg := map[string]string{"username": "alice", "content": "hello"}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should receive the broadcast back.
	var got Message
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("read: %v", err)
	}

	if got.Username != "alice" || got.Content != "hello" || got.Website != "example.com" {
		t.Errorf("unexpected message: %+v", got)
	}
}

func TestWebSocketRoomIsolation(t *testing.T) {
	_, ts := newTestServer(t)

	c1 := wsConnect(t, ts, "site-a.com")
	c2 := wsConnect(t, ts, "site-b.com")

	// Give connections time to register.
	time.Sleep(50 * time.Millisecond)

	// Send from site-a.
	if err := c1.WriteJSON(map[string]string{"username": "alice", "content": "hi"}); err != nil {
		t.Fatal(err)
	}

	// c1 should get the message.
	var got Message
	if err := c1.ReadJSON(&got); err != nil {
		t.Fatalf("c1 read: %v", err)
	}
	if got.Content != "hi" {
		t.Errorf("c1 got %q, want %q", got.Content, "hi")
	}

	// c2 should NOT get the message (different room).
	c2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var got2 Message
	err := c2.ReadJSON(&got2)
	if err == nil {
		t.Error("c2 should not have received message from different room")
	}
}

func TestMessagesEndpoint(t *testing.T) {
	_, ts := newTestServer(t)

	conn := wsConnect(t, ts, "test.com")

	// Send a message.
	conn.WriteJSON(map[string]string{"username": "bob", "content": "stored"})

	// Wait for broadcast to complete.
	var discard Message
	conn.ReadJSON(&discard)

	// Fetch via REST.
	resp, err := http.Get(ts.URL + "/api/messages?website=test.com")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	var msgs []Message
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		t.Fatal(err)
	}

	if len(msgs) != 1 || msgs[0].Content != "stored" {
		t.Errorf("unexpected messages: %+v", msgs)
	}
}

func TestMessagesEndpointMissingParam(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var stats Stats
	json.NewDecoder(resp.Body).Decode(&stats)

	if stats.ActiveRooms != 0 || stats.ActiveClients != 0 {
		t.Errorf("expected empty stats, got %+v", stats)
	}
}

func TestCORSHeaders(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected CORS origin *, got %q", origin)
	}
}

func TestFrontendServed(t *testing.T) {
	_, ts := newTestServer(t)

	// Root should serve the chat UI.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}
}

func TestWidgetJSServed(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/widget.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /widget.js, got %d", resp.StatusCode)
	}
}
