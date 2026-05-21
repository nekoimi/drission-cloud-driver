package v1

import (
	ws "github.com/nekoimi/go-project-template/internal/websocket"
)

type WSHandler = ws.WSHandler

func NewWSHandler(h *ws.WSHandler) *WSHandler {
	return h
}
