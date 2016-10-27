package server

import (
	"time"

	"golang.org/x/net/context"
)

var defaultTimeout = 15 * time.Second

// getContext returns a context for making requests to twilio. If the context
// has an existing Deadline, the new context leaves timeToLeave time for the
// context to complete. Otherwise we return a context initialized with the
// defaultTimeout.
func getContext(ctx context.Context, timeToLeave time.Duration) (context.Context, context.CancelFunc) {
	if timeToLeave < 0 {
		panic("invalid timeToLeave")
	}
	deadline, ok := ctx.Deadline()
	if ok {
		return context.WithDeadline(ctx, deadline.Add(-timeToLeave))
	}
	return context.WithTimeout(ctx, defaultTimeout)
}
