package pagechat

import (
	"testing"
	"time"
)

func TestHubRegisterUnregister(t *testing.T) {
	h := newHub(time.Hour, 100, false)

	c := &client{
		hub:     h,
		send:    make(chan Message, sendBufSize),
		website: "example.com",
	}

	h.register(c)

	h.mu.RLock()
	room, ok := h.rooms["example.com"]
	h.mu.RUnlock()

	if !ok || len(room) != 1 {
		t.Fatal("expected 1 client in room after register")
	}

	h.unregister(c)

	h.mu.RLock()
	_, ok = h.rooms["example.com"]
	h.mu.RUnlock()

	if ok {
		t.Fatal("expected room to be removed after last client unregisters")
	}
}

func TestHubDoubleUnregister(t *testing.T) {
	h := newHub(time.Hour, 100, false)

	c := &client{
		hub:     h,
		send:    make(chan Message, sendBufSize),
		website: "example.com",
	}

	h.register(c)
	h.unregister(c)
	// Second unregister should not panic.
	h.unregister(c)
}

func TestHubBroadcast(t *testing.T) {
	h := newHub(time.Hour, 100, false)

	c1 := &client{hub: h, send: make(chan Message, sendBufSize), website: "example.com"}
	c2 := &client{hub: h, send: make(chan Message, sendBufSize), website: "example.com"}
	c3 := &client{hub: h, send: make(chan Message, sendBufSize), website: "other.com"}

	h.register(c1)
	h.register(c2)
	h.register(c3)

	msg := Message{
		Website:   "example.com",
		Username:  "alice",
		Content:   "hello world",
		Timestamp: time.Now(),
	}

	h.broadcast(msg)

	// c1 and c2 should receive the message.
	select {
	case got := <-c1.send:
		if got.Content != "hello world" {
			t.Errorf("c1 got content %q, want %q", got.Content, "hello world")
		}
	default:
		t.Error("c1 did not receive message")
	}

	select {
	case got := <-c2.send:
		if got.Content != "hello world" {
			t.Errorf("c2 got content %q, want %q", got.Content, "hello world")
		}
	default:
		t.Error("c2 did not receive message")
	}

	// c3 is in a different room and should not receive the message.
	select {
	case <-c3.send:
		t.Error("c3 should not have received message from different room")
	default:
		// expected
	}
}

func TestHubBroadcastWithContentFilter(t *testing.T) {
	h := newHub(time.Hour, 100, true)

	c := &client{hub: h, send: make(chan Message, sendBufSize), website: "example.com"}
	h.register(c)

	msg := Message{
		Website:   "example.com",
		Username:  "bob",
		Content:   "this is shit",
		Timestamp: time.Now(),
	}

	h.broadcast(msg)

	select {
	case got := <-c.send:
		if got.Content == "this is shit" {
			t.Error("content filter should have censored the message")
		}
	default:
		t.Error("client did not receive message")
	}
}

func TestHubMessageHistory(t *testing.T) {
	h := newHub(time.Hour, 100, false)

	c := &client{hub: h, send: make(chan Message, sendBufSize), website: "example.com"}
	h.register(c)

	for i := 0; i < 5; i++ {
		h.broadcast(Message{
			Website:   "example.com",
			Username:  "alice",
			Content:   "msg",
			Timestamp: time.Now(),
		})
	}

	msgs := h.getMessages("example.com")
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}

	// Non-existent room returns empty slice.
	msgs = h.getMessages("nonexistent.com")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for nonexistent room, got %d", len(msgs))
	}
}

func TestHubMaxMessages(t *testing.T) {
	h := newHub(time.Hour, 3, false)

	c := &client{hub: h, send: make(chan Message, sendBufSize), website: "example.com"}
	h.register(c)

	for i := 0; i < 10; i++ {
		h.broadcast(Message{
			Website:   "example.com",
			Username:  "alice",
			Content:   "msg",
			Timestamp: time.Now(),
		})
	}

	msgs := h.getMessages("example.com")
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages (max), got %d", len(msgs))
	}
}

func TestHubCleanupExpired(t *testing.T) {
	h := newHub(time.Millisecond*100, 100, false)

	c := &client{hub: h, send: make(chan Message, sendBufSize), website: "example.com"}
	h.register(c)

	h.broadcast(Message{
		Website:   "example.com",
		Username:  "alice",
		Content:   "old message",
		Timestamp: time.Now().Add(-time.Second), // already expired
	})

	h.cleanupExpired()

	msgs := h.getMessages("example.com")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cleanup, got %d", len(msgs))
	}
}

func TestHubStats(t *testing.T) {
	h := newHub(time.Hour, 100, false)
	start := time.Now()

	c1 := &client{hub: h, send: make(chan Message, sendBufSize), website: "a.com"}
	c2 := &client{hub: h, send: make(chan Message, sendBufSize), website: "a.com"}
	c3 := &client{hub: h, send: make(chan Message, sendBufSize), website: "b.com"}

	h.register(c1)
	h.register(c2)
	h.register(c3)

	h.broadcast(Message{Website: "a.com", Username: "u", Content: "m", Timestamp: time.Now()})

	stats := h.stats(start)
	if stats.ActiveRooms != 2 {
		t.Errorf("expected 2 rooms, got %d", stats.ActiveRooms)
	}
	if stats.ActiveClients != 3 {
		t.Errorf("expected 3 clients, got %d", stats.ActiveClients)
	}
	if stats.StoredMessages != 1 {
		t.Errorf("expected 1 message, got %d", stats.StoredMessages)
	}
}
