package core

import (
	"context"
	"sync"
	"time"
)

// TransferLimiter belongs to one file tab. Updates wake active waits immediately.
type TransferLimiter struct {
	mu      sync.Mutex
	rate    int64
	changed chan struct{}
}
type transferLimitKey struct{}

func (l *TransferLimiter) Set(bytesPerSecond int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rate = max(0, bytesPerSecond)
	if l.changed != nil {
		close(l.changed)
	}
	l.changed = make(chan struct{})
}
func (l *TransferLimiter) Rate() int64 { l.mu.Lock(); defer l.mu.Unlock(); return l.rate }
func WithTransferLimiter(ctx context.Context, l *TransferLimiter) context.Context {
	return context.WithValue(ctx, transferLimitKey{}, l)
}
func waitTransfer(ctx context.Context, n int) error {
	l, _ := ctx.Value(transferLimitKey{}).(*TransferLimiter)
	if l == nil {
		return ctx.Err()
	}
	remaining := float64(n)
	for remaining > 0 {
		l.mu.Lock()
		rate := l.rate
		if l.changed == nil {
			l.changed = make(chan struct{})
		}
		changed := l.changed
		l.mu.Unlock()
		if rate == 0 {
			return ctx.Err()
		}
		started := time.Now()
		// Bound each wait to avoid overflow with unusually low rates.
		duration := min(time.Second, time.Duration(remaining/float64(rate)*float64(time.Second)))
		timer := time.NewTimer(duration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-changed:
			timer.Stop()
		case <-timer.C:
		}
		remaining -= time.Since(started).Seconds() * float64(rate)
	}
	return ctx.Err()
}

func snapshotTransferLimit(ctx context.Context) context.Context {
	if l, ok := ctx.Value(transferLimitKey{}).(*TransferLimiter); ok {
		fixed := &TransferLimiter{}
		fixed.Set(l.Rate())
		return WithTransferLimiter(ctx, fixed)
	}
	return ctx
}
