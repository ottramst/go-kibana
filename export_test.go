package kibana

import (
	"context"
	"iter"
	"time"
)

// Paginate exposes paginate to the tests of package kibana_test.
func Paginate[T any](ctx context.Context, from, size int, fetch func(ctx context.Context, from, size int) ([]T, int, error)) iter.Seq2[T, error] {
	return paginate(ctx, from, size, fetch)
}

// CheckResponse exposes checkResponse to the tests of package kibana_test.
var CheckResponse = checkResponse

// AddQuery exposes addQuery to the tests of package kibana_test.
var AddQuery = addQuery

// WithRetrySleep replaces the function the transport of WithRetry waits
// with, so that tests don't sleep.
func WithRetrySleep(sleep func(ctx context.Context, d time.Duration) error) ClientOption {
	return func(c *Client) error {
		c.retrySleep = sleep
		return nil
	}
}

// RetryAfter exposes retryAfter to the tests of package kibana_test.
var RetryAfter = retryAfter
