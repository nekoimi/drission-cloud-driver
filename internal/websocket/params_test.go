package websocket

import (
	"testing"
	"time"

	"github.com/nekoimi/go-project-template/internal/config"
)

func TestNewConnParams_defaults(t *testing.T) {
	p := newConnParams(config.WebsocketConfig{})
	if p.readBufferSize != 1024 || p.writeBufferSize != 1024 {
		t.Fatalf("buffer defaults: %+v", p)
	}
	if p.maxMessageSize != 4096 {
		t.Fatalf("max size: %d", p.maxMessageSize)
	}
	if p.pingPeriod >= p.readWait || p.pingPeriod <= 0 {
		t.Fatalf("ping %v read %v", p.pingPeriod, p.readWait)
	}
}

func TestNewConnParams_respectsPing(t *testing.T) {
	p := newConnParams(config.WebsocketConfig{
		ReadWait:   60 * time.Second,
		PingPeriod: 30 * time.Second,
	})
	if p.pingPeriod != 30*time.Second {
		t.Fatalf("want 30s ping, got %v", p.pingPeriod)
	}
}
