package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allowing all origins for development
	},
}

// Client represents a connected user
type Client struct {
	UserUUID string
	Conn     *websocket.Conn
	Send     chan []byte
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[string][]*Client // Map user UUID to their active connections
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

// Message defines the structure of events sent over WebSocket
type Message struct {
	TargetUsers []string `json:"target_users"` // List of user UUIDs to notify
	Payload     any      `json:"payload"`      // Data to send (e.g., { type: "update" })
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string][]*Client),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserUUID] = append(h.clients[client.UserUUID], client)
			h.mu.Unlock()
			log.Printf("User %s registered. Total connections: %d", client.UserUUID, len(h.clients[client.UserUUID]))

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.UserUUID]; ok {
				for i, c := range clients {
					if c == client {
						h.clients[client.UserUUID] = append(clients[:i], clients[i+1:]...)
						break
					}
				}
				if len(h.clients[client.UserUUID]) == 0 {
					delete(h.clients, client.UserUUID)
				}
			}
			close(client.Send)
			h.mu.Unlock()
			log.Printf("User %s unregistered.", client.UserUUID)

		case message := <-h.broadcast:
			payload, _ := json.Marshal(message.Payload)
			h.mu.Lock()
			for _, userUUID := range message.TargetUsers {
				if clients, ok := h.clients[userUUID]; ok {
					for _, client := range clients {
						select {
						case client.Send <- payload:
						default:
							// Cleanup stalled connection
							// Logic could be added here
						}
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// ServeWs handles websocket requests from the peer.
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request, userUUID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{UserUUID: userUUID, Conn: conn, Send: make(chan []byte, 256)}
	h.register <- client

	// Start reader and writer
	go client.writePump()
	go client.readPump(h)
}

func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.Conn.Close()
	}()
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.Conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		}
	}
}

// BroadcastToUsers sends a message to a list of users
func (h *Hub) BroadcastToUsers(userUUIDs []string, payload any) {
	h.broadcast <- Message{
		TargetUsers: userUUIDs,
		Payload:     payload,
	}
}
