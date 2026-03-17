package pagechat

import (
	"log/slog"
	"sync"
	"time"

	goaway "github.com/TwiN/go-away"
)

// hub maintains active rooms and coordinates message delivery.
type hub struct {
	mu       sync.RWMutex
	rooms    map[string]map[*client]bool
	messages map[string][]Message

	messageTTL    time.Duration
	maxMessages   int
	contentFilter bool

	done chan struct{}
}

func newHub(messageTTL time.Duration, maxMessages int, contentFilter bool) *hub {
	if messageTTL == 0 {
		messageTTL = 24 * time.Hour
	}
	if maxMessages == 0 {
		maxMessages = 1000
	}
	return &hub{
		rooms:         make(map[string]map[*client]bool),
		messages:      make(map[string][]Message),
		messageTTL:    messageTTL,
		maxMessages:   maxMessages,
		contentFilter: contentFilter,
		done:          make(chan struct{}),
	}
}

// register adds a client to its room.
func (h *hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[c.website] == nil {
		h.rooms[c.website] = make(map[*client]bool)
	}
	h.rooms[c.website][c] = true

	slog.Info("client connected",
		"website", c.website,
		"room_size", len(h.rooms[c.website]),
	)
}

// unregister removes a client from its room and closes its send channel.
func (h *hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[c.website]
	if !ok {
		return
	}
	if _, exists := room[c]; !exists {
		return
	}

	delete(room, c)
	close(c.send)

	slog.Info("client disconnected",
		"website", c.website,
		"room_size", len(room),
	)

	if len(room) == 0 {
		delete(h.rooms, c.website)
	}
}

// broadcast sends a message to all clients in the same room.
func (h *hub) broadcast(msg Message) {
	if h.contentFilter {
		msg.Content = goaway.Censor(msg.Content)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Store message
	h.messages[msg.Website] = append(h.messages[msg.Website], msg)
	if len(h.messages[msg.Website]) > h.maxMessages {
		// Trim oldest messages
		excess := len(h.messages[msg.Website]) - h.maxMessages
		h.messages[msg.Website] = h.messages[msg.Website][excess:]
	}

	// Deliver to room
	room := h.rooms[msg.Website]
	for c := range room {
		select {
		case c.send <- msg:
		default:
			// Client too slow; disconnect it.
			delete(room, c)
			close(c.send)
		}
	}
}

// getMessages returns a copy of stored messages for a website.
func (h *hub) getMessages(website string) []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	msgs := h.messages[website]
	if len(msgs) == 0 {
		return []Message{}
	}

	result := make([]Message, len(msgs))
	copy(result, msgs)
	return result
}

// Stats contains server statistics.
type Stats struct {
	ActiveRooms    int `json:"active_rooms"`
	ActiveClients  int `json:"active_clients"`
	StoredMessages int `json:"stored_messages"`
	Uptime         string `json:"uptime"`
}

func (h *hub) stats(startTime time.Time) Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := 0
	for _, room := range h.rooms {
		clients += len(room)
	}

	msgs := 0
	for _, m := range h.messages {
		msgs += len(m)
	}

	return Stats{
		ActiveRooms:    len(h.rooms),
		ActiveClients:  clients,
		StoredMessages: msgs,
		Uptime:         time.Since(startTime).Truncate(time.Second).String(),
	}
}

// startCleanup periodically removes expired messages.
func (h *hub) startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.cleanupExpired()
			case <-h.done:
				return
			}
		}
	}()
}

func (h *hub) cleanupExpired() {
	cutoff := time.Now().Add(-h.messageTTL)

	h.mu.Lock()
	defer h.mu.Unlock()

	for website, msgs := range h.messages {
		i := 0
		for i < len(msgs) && msgs[i].Timestamp.Before(cutoff) {
			i++
		}
		if i > 0 {
			h.messages[website] = msgs[i:]
		}
		if len(h.messages[website]) == 0 {
			delete(h.messages, website)
		}
	}
}

func (h *hub) stop() {
	close(h.done)
}
