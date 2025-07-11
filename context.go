package eventbus

import (
	"context"

	"github.com/facebookincubator/go-belt/tool/logger"
)

var LoggingEnabled = false

func isTraceEnabled(ctx context.Context) bool {
	if !LoggingEnabled {
		return false
	}
	return logger.FromCtx(ctx).Level() >= logger.LevelTrace
}
