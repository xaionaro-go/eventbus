package eventbus

import (
	"context"

	"github.com/facebookincubator/go-belt/tool/logger"
)

// TODO: move to a separate package
type chanLocker chan struct{}

func (l chanLocker) Lock(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		if isTraceEnabled(ctx) {
			logger.Tracef(ctx, "context is closed")
		}
		return false
	case l <- struct{}{}:
		return true
	}
}

func (l chanLocker) TryLock(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		if isTraceEnabled(ctx) {
			logger.Tracef(ctx, "context is closed")
		}
		return false
	case l <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l chanLocker) Unlock() {
	select {
	case <-l:
	default:
		panic("not locked!")
	}
}
