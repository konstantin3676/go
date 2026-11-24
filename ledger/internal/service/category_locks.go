package service

import (
	"context"
	"sync"
)

// categoryLocks serializes budget checks, inserts and budget updates within a category.
type categoryLocks struct {
	mu      sync.Mutex
	entries map[string]*categoryLock
}

type categoryLock struct {
	held chan struct{}
	refs int // current holder and waiters; protected by categoryLocks.mu
}

func (l *categoryLocks) acquire(ctx context.Context, category string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[string]*categoryLock)
	}
	entry := l.entries[category]
	if entry == nil {
		entry = &categoryLock{held: make(chan struct{}, 1)}
		l.entries[category] = entry
	}
	entry.refs++
	l.mu.Unlock()

	forget := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.entries, category)
		}
	}
	select {
	case <-ctx.Done():
		forget()
		return nil, ctx.Err()
	case entry.held <- struct{}{}:
		release := func() {
			<-entry.held
			forget()
		}
		// The lock and cancellation can both be ready in the select.
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		return release, nil
	}
}
