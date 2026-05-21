package websocket

import (
	"time"

	"github.com/nekoimi/go-project-template/internal/config"
)

// connParams 由配置推导，供 Upgrader 与 Client 使用；零值字段会使用与历史常量一致的默认。
type connParams struct {
	readBufferSize  int
	writeBufferSize int
	writeWait       time.Duration
	readWait        time.Duration
	pingPeriod      time.Duration
	maxMessageSize  int64
}

func newConnParams(c config.WebsocketConfig) connParams {
	p := connParams{
		readBufferSize:  c.ReadBufferSize,
		writeBufferSize: c.WriteBufferSize,
		writeWait:       c.WriteWait,
		readWait:        c.ReadWait,
		maxMessageSize:  c.MaxMessageSize,
	}
	if p.readBufferSize <= 0 {
		p.readBufferSize = 1024
	}
	if p.writeBufferSize <= 0 {
		p.writeBufferSize = 1024
	}
	if p.writeWait <= 0 {
		p.writeWait = 10 * time.Second
	}
	if p.readWait <= 0 {
		p.readWait = 60 * time.Second
	}
	if p.maxMessageSize <= 0 {
		p.maxMessageSize = 4096
	}

	if c.PingPeriod > 0 {
		p.pingPeriod = c.PingPeriod
	} else {
		p.pingPeriod = p.readWait * 9 / 10
	}
	if p.pingPeriod <= 0 {
		p.pingPeriod = 54 * time.Second
	}
	if p.pingPeriod >= p.readWait {
		p.pingPeriod = p.readWait * 9 / 10
		if p.pingPeriod <= 0 {
			p.pingPeriod = time.Second
		}
	}
	return p
}
