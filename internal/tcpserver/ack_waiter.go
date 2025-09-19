package tcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ACKWaiter struct {
	waiters map[string]chan *ACKMessage
	mu      sync.RWMutex
}

func NewACKWaiter() *ACKWaiter {
	return &ACKWaiter{
		waiters: make(map[string]chan *ACKMessage),
	}
}

func (aw *ACKWaiter) Wait(cmdID string, timeout time.Duration) (*ACKMessage, error) {
	ch := make(chan *ACKMessage, 1)

	aw.mu.Lock()
	aw.waiters[cmdID] = ch
	aw.mu.Unlock()

	defer func() {
		aw.mu.Lock()
		delete(aw.waiters, cmdID)
		close(ch)
		aw.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case ack := <-ch:
		return ack, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("ack timeout for command %s", cmdID)
	}
}

func (aw *ACKWaiter) Notify(cmdID string, ack *ACKMessage) {
	aw.mu.RLock()
	ch, exists := aw.waiters[cmdID]
	aw.mu.RUnlock()

	if exists {
		select {
		case ch <- ack:
		default:
		}
	}
}

func (aw *ACKWaiter) Cancel(cmdID string) {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if ch, exists := aw.waiters[cmdID]; exists {
		close(ch)
		delete(aw.waiters, cmdID)
	}
}