package websocket

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Client struct {
	manager  *Manager
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	logger   *zap.Logger
	params   connParams
}

func newClient(manager *Manager, conn *websocket.Conn, userID string, logger *zap.Logger, params connParams) *Client {
	return &Client{
		manager: manager,
		conn:    conn,
		send:    make(chan []byte, 256),
		userID:  userID,
		logger:  logger,
		params:  params,
	}
}

// ReadPump reads messages from the WebSocket connection and dispatches them.
func (c *Client) ReadPump() {
	defer func() {
		c.manager.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(c.params.maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.params.readWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.params.readWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("websocket read error", zap.String("userID", c.userID), zap.Error(err))
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.logger.Warn("invalid message format", zap.String("userID", c.userID), zap.Error(err))
			continue
		}

		msg.From = c.userID
		msg.Time = time.Now().UnixMilli()

		c.handleMessage(&msg)
	}
}

// WritePump writes messages to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(c.params.pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.params.writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Warn("websocket write error", zap.String("userID", c.userID), zap.Error(err))
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.params.writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case MessageTypeChat:
		if msg.To != "" {
			c.manager.SendToUser(msg.To, msg)
		} else {
			c.manager.Broadcast(msg)
		}
	case MessageTypeNotify:
		if msg.To != "" {
			c.manager.SendToUser(msg.To, msg)
		}
	default:
		c.logger.Debug("unknown message type", zap.String("type", msg.Type))
	}
}
