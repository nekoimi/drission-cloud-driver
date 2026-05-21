package v1

import (
	ws "github.com/nekoimi/drission-cloud-driver/internal/websocket"
)

type WSHandler = ws.WSHandler

func NewWSHandler(h *ws.WSHandler) *WSHandler {
	return h
}
