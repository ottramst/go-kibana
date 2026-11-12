package kibana

import (
	"context"
	"iter"
)

// Paginate exposes paginate to the tests of package kibana_test.
func Paginate[T any](ctx context.Context, from, size int, fetch func(ctx context.Context, from, size int) ([]T, int, error)) iter.Seq2[T, error] {
	return paginate(ctx, from, size, fetch)
}
