package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Manager manages all WebSocket connections (Hub pattern).
type Manager struct {
	clients    map[string]*Client // userID -> client
	rooms      map[string]map[string]*Client // roomID -> (userID -> client)
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	logger     *zap.Logger
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		rooms:      make(map[string]map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		logger:     logger,
	}
}

// Run starts the manager loop. Should be called in a goroutine.
func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			if old, exists := m.clients[client.userID]; exists {
				close(old.send)
				delete(m.clients, client.userID)
			}
			m.clients[client.userID] = client
			m.mu.Unlock()
			m.logger.Info("websocket client registered", zap.String("userID", client.userID))

		case client := <-m.unregister:
			m.mu.Lock()
			if cur, exists := m.clients[client.userID]; exists && cur == client {
				delete(m.clients, client.userID)
				close(client.send)
			}
			// Remove from all rooms
			for roomID, room := range m.rooms {
				if _, exists := room[client.userID]; exists {
					delete(room, client.userID)
					if len(room) == 0 {
						delete(m.rooms, roomID)
					}
				}
			}
			m.mu.Unlock()
			m.logger.Info("websocket client unregistered", zap.String("userID", client.userID))
		}
	}
}

// SendToUser sends a message to a specific user.
func (m *Manager) SendToUser(userID string, msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		m.logger.Error("failed to marshal message", zap.Error(err))
		return
	}

	m.mu.RLock()
	client, exists := m.clients[userID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	select {
	case client.send <- data:
	default:
		m.logger.Warn("send channel full", zap.String("userID", userID))
	}
}

// Broadcast sends a message to all connected clients.
func (m *Manager) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		m.logger.Error("failed to marshal message", zap.Error(err))
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		select {
		case client.send <- data:
		default:
			m.logger.Warn("send channel full", zap.String("userID", client.userID))
		}
	}
}

// BroadcastExcept sends a message to all clients except the specified user.
func (m *Manager) BroadcastExcept(excludeUserID string, msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		m.logger.Error("failed to marshal message", zap.Error(err))
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for uid, client := range m.clients {
		if uid == excludeUserID {
			continue
		}
		select {
		case client.send <- data:
		default:
			m.logger.Warn("send channel full", zap.String("userID", uid))
		}
	}
}

// JoinRoom adds a user to a room.
func (m *Manager) JoinRoom(roomID string, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[roomID]; !exists {
		m.rooms[roomID] = make(map[string]*Client)
	}
	m.rooms[roomID][client.userID] = client
}

// LeaveRoom removes a user from a room.
func (m *Manager) LeaveRoom(roomID string, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, exists := m.rooms[roomID]; exists {
		delete(room, userID)
		if len(room) == 0 {
			delete(m.rooms, roomID)
		}
	}
}

// SendToRoom sends a message to all clients in a room.
func (m *Manager) SendToRoom(roomID string, msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		m.logger.Error("failed to marshal message", zap.Error(err))
		return
	}

	m.mu.RLock()
	room, exists := m.rooms[roomID]
	if !exists {
		m.mu.RUnlock()
		return
	}

	for _, client := range room {
		select {
		case client.send <- data:
		default:
			m.logger.Warn("send channel full", zap.String("userID", client.userID))
		}
	}
	m.mu.RUnlock()
}

// ClientCount returns the number of connected clients.
func (m *Manager) ClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// Shutdown gracefully closes all WebSocket connections.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for userID, client := range m.clients {
		// Send close frame
		_ = client.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"))
		close(client.send)
		delete(m.clients, userID)
	}
	m.rooms = make(map[string]map[string]*Client)
	m.logger.Info("websocket manager shutdown complete")
}
