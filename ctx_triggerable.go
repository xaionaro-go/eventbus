package eventbus

import (
	"context"
)

// TODO: move to a separate package
type triggerable struct {
	context.Context
	CancelFunc context.CancelFunc
}

func newTriggerable(ctx context.Context) *triggerable {
	ctx, cancelFn := context.WithCancel(ctx)
	return &triggerable{
		Context:    ctx,
		CancelFunc: cancelFn,
	}
}

func (ctx *triggerable) Trigger() {
	ctx.CancelFunc()
}
