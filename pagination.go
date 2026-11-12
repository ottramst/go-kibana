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

// pageNumberFunc fetches page number page, starting at 1, of perPage items,
// and returns the items and the total number of items.
type pageNumberFunc[T any] func(ctx context.Context, page, perPage int) (items []T, total int, err error)

// paginatePages is paginate for endpoints that take page numbers instead of
// offsets, starting at page number page (1 if zero). A page shorter than
// perPage before the last one, because items were deleted while iterating,
// ends the iteration: the next offset would not start a page.
func paginatePages[T any](ctx context.Context, page, perPage int, fetch pageNumberFunc[T]) iter.Seq2[T, error] {
	if perPage <= 0 {
		perPage = defaultPageSize
	}
	from := 0
	if page > 1 {
		from = (page - 1) * perPage
	}
	return paginate(ctx, from, perPage, func(ctx context.Context, from, size int) ([]T, int, error) {
		if from%size != 0 {
			return nil, 0, nil
		}
		return fetch(ctx, from/size+1, size)
	})
}
