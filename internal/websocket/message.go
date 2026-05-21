package websocket

import "time"

const (
	MessageTypeChat   = "chat"
	MessageTypeNotify = "notify"
	MessageTypeSystem = "system"
)

type Message struct {
	Type    string      `json:"type"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
	Content interface{} `json:"content"`
	Time    int64       `json:"time"`
}

func NewMessage(msgType string, from string, content interface{}) *Message {
	return &Message{
		Type:    msgType,
		From:    from,
		Content: content,
		Time:    time.Now().UnixMilli(),
	}
}
