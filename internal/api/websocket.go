package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// WSEvent represents a WebSocket event
type WSEvent struct {
	Type      string      `json:"type"`      // "member_status", "activity", "task_update"
	Team      string      `json:"team"`
	MemberID  string      `json:"member_id,omitempty"`
	Status    string      `json:"status,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// WSHub manages WebSocket connections
type WSHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan WSEvent
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

// NewWSHub creates a new WebSocket hub
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan WSEvent, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run starts the hub's main loop
func (h *WSHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.RLock()
			data, _ := json.Marshal(event)
			for conn := range h.clients {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends an event to all connected clients
func (h *WSHub) Broadcast(event WSEvent) {
	event.Timestamp = time.Now()
	h.mu.RLock()
	clientCount := len(h.clients)
	h.mu.RUnlock()

	select {
	case h.broadcast <- event:
		// Debug: log broadcast
		if event.Type == "chat" || event.Type == "team_update" {
			println("[WS] Broadcasting", event.Type, "to", clientCount, "clients, team:", event.Team)
		}
	default:
		// Channel full, skip
		println("[WS] WARNING: broadcast channel full, skipping", event.Type)
	}
}

// BroadcastMemberStatus sends a member status update
func (h *WSHub) BroadcastMemberStatus(team, memberID, status, message string) {
	h.Broadcast(WSEvent{
		Type:     "member_status",
		Team:     team,
		MemberID: memberID,
		Status:   status,
		Message:  message,
	})
}

// BroadcastActivity sends an activity event
func (h *WSHub) BroadcastActivity(team, memberID, message string, data interface{}) {
	h.Broadcast(WSEvent{
		Type:     "activity",
		Team:     team,
		MemberID: memberID,
		Message:  message,
		Data:     data,
	})
}

// BroadcastChat sends a chat message event
func (h *WSHub) BroadcastChat(team, memberID, msgType, from, content string) {
	h.Broadcast(WSEvent{
		Type:     "chat",
		Team:     team,
		MemberID: memberID,
		Message:  content,
		Data: map[string]string{
			"from":     from,
			"msg_type": msgType, // "user" or "agent"
		},
	})
}

// BroadcastTeamUpdate sends a team update event (created, deleted, started, stopped)
func (h *WSHub) BroadcastTeamUpdate(action, teamName string, data interface{}) {
	h.Broadcast(WSEvent{
		Type:    "team_update",
		Team:    teamName,
		Message: action,
		Data:    data,
	})
}

// BroadcastSpecUpdate sends a spec update event (created, updated, deleted)
func (h *WSHub) BroadcastSpecUpdate(action, specName string, data interface{}) {
	h.Broadcast(WSEvent{
		Type:    "spec_update",
		Team:    specName, // Using team field for spec name
		Message: action,
		Data:    data,
	})
}

// BroadcastSettingsUpdate sends a settings/provider update event
func (h *WSHub) BroadcastSettingsUpdate(section string, data interface{}) {
	h.Broadcast(WSEvent{
		Type:    "settings_update",
		Message: section,
		Data:    data,
	})
}

// HandleWS handles WebSocket connections
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	s.wsHub.register <- conn

	// Keep connection alive and handle incoming messages
	go func() {
		defer func() {
			s.wsHub.unregister <- conn
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// GetWSHub returns the WebSocket hub
func (s *Server) GetWSHub() *WSHub {
	return s.wsHub
}
