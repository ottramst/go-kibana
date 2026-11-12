package kibana

import (
	"context"
	"iter"
)

// defaultPageSize is the page size of the iterators when none is given.
const defaultPageSize = 100

// pageFunc fetches the page of size items that starts at offset from, and
// returns the items and the total number of items.
type pageFunc[T any] func(ctx context.Context, from, size int) (items []T, total int, err error)

// paginate returns an iterator over all items of an offset-paginated
// endpoint, starting at offset from. Pages are fetched lazily, and ranging
// over the iterator again starts over.
//
// The iterator yields an error, with the zero item, when a page fails or ctx
// is done, and then stops. Offset pagination can skip or repeat items when
// they change while iterating, and Elasticsearch-backed endpoints reject
// offsets beyond 10,000 items.
func paginate[T any](ctx context.Context, from, size int, fetch pageFunc[T]) iter.Seq2[T, error] {
	if size <= 0 {
		size = defaultPageSize
	}
	return func(yield func(T, error) bool) {
		var zero T
		for offset := from; ; {
			if err := ctx.Err(); err != nil {
				yield(zero, err)
				return
			}
			items, total, err := fetch(ctx, offset, size)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			offset += len(items)
			// An empty page also ends the iteration, in case the total is
			// wrong, to avoid looping forever.
			if len(items) == 0 || offset >= total {
				return
			}
		}
	}
}
