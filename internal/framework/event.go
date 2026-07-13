package framework

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Topic     string
	Payload   any
	Metadata  map[string]string
	CreatedAt time.Time
}

type EventHandler func(ctx context.Context, event Event) error

type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

func (b *EventBus) Subscribe(topic string, handler EventHandler) {
	if b == nil || topic == "" || handler == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}

func (b *EventBus) Publish(ctx context.Context, event Event) error {
	if b == nil || event.Topic == "" {
		return nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	b.mu.RLock()
	handlers := append([]EventHandler(nil), b.handlers[event.Topic]...)
	b.mu.RUnlock()

	var errs []error
	for i, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s handler %d: %w", event.Topic, i, err))
		}
	}
	return errors.Join(errs...)
}
