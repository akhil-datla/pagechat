package pagechat

import "time"

// Message represents a chat message exchanged in a room.
type Message struct {
	Website   string    `json:"website"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// incomingMessage is the JSON shape clients send over WebSocket.
type incomingMessage struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}
