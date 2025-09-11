package s3

import (
	"context"
	"time"
)

// getContextWithTimeout returns a context with timeout
func getContextWithTimeout(timeout time.Duration, ctx context.Context) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}
