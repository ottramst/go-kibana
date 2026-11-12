package kibana_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

// pager serves items in pages and records the requested offsets.
type pager struct {
	items   []int
	total   int // reported total; len(items) if zero
	failAt  int // offset at which to fail; -1 for never
	offsets []int
}

func (p *pager) fetch(_ context.Context, from, size int) ([]int, int, error) {
	p.offsets = append(p.offsets, from)
	if from == p.failAt {
		return nil, 0, errors.New("page failed")
	}
	total := p.total
	if total == 0 {
		total = len(p.items)
	}
	end := min(from+size, len(p.items))
	if from >= end {
		return nil, total, nil
	}
	return p.items[from:end], total, nil
}

func collect(t *testing.T, seq func(func(int, error) bool)) ([]int, error) {
	t.Helper()
	var got []int
	for v, err := range seq {
		if err != nil {
			return got, err
		}
		got = append(got, v)
	}
	return got, nil
}

func TestPaginate(t *testing.T) {
	p := &pager{items: []int{1, 2, 3, 4, 5}, failAt: -1}
	seq := kibana.Paginate(context.Background(), 0, 2, p.fetch)
	got, err := collect(t, seq)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if want := []int{1, 2, 3, 4, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if want := []int{0, 2, 4}; !reflect.DeepEqual(p.offsets, want) {
		t.Errorf("offsets = %v, want %v", p.offsets, want)
	}

	// Ranging again starts over.
	p.offsets = nil
	if got, _ := collect(t, seq); len(got) != 5 || p.offsets[0] != 0 {
		t.Errorf("second range: items = %v, offsets = %v", got, p.offsets)
	}
}

func TestPaginate_fromAndDefaultSize(t *testing.T) {
	items := make([]int, 250)
	for i := range items {
		items[i] = i
	}
	p := &pager{items: items, failAt: -1}
	got, err := collect(t, kibana.Paginate(context.Background(), 10, 0, p.fetch))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(got) != 240 || got[0] != 10 {
		t.Errorf("got %d items starting at %v, want 240 starting at 10", len(got), got[0])
	}
	if want := []int{10, 110, 210}; !reflect.DeepEqual(p.offsets, want) {
		t.Errorf("offsets = %v, want %v (page size 100)", p.offsets, want)
	}
}

func TestPaginate_earlyBreak(t *testing.T) {
	p := &pager{items: []int{1, 2, 3, 4, 5}, failAt: -1}
	n := 0
	for _, err := range kibana.Paginate(context.Background(), 0, 2, p.fetch) {
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		n++
		if n == 3 {
			break
		}
	}
	if want := []int{0, 2}; !reflect.DeepEqual(p.offsets, want) {
		t.Errorf("offsets = %v, want %v (no fetch after break)", p.offsets, want)
	}
}

func TestPaginate_error(t *testing.T) {
	p := &pager{items: []int{1, 2, 3, 4, 5}, failAt: 2}
	var got []int
	var errs int
	for v, err := range kibana.Paginate(context.Background(), 0, 2, p.fetch) {
		if err != nil {
			errs++
			continue
		}
		got = append(got, v)
	}
	if errs != 1 || !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("items = %v, errors = %d; want [1 2] and one error", got, errs)
	}
	if want := []int{0, 2}; !reflect.DeepEqual(p.offsets, want) {
		t.Errorf("offsets = %v, want %v", p.offsets, want)
	}
}

func TestPaginate_canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &pager{items: []int{1, 2, 3, 4, 5}, failAt: -1}
	var got []int
	var gotErr error
	for v, err := range kibana.Paginate(ctx, 0, 2, p.fetch) {
		if err != nil {
			gotErr = err
			break
		}
		got = append(got, v)
		cancel()
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", gotErr)
	}
	if len(p.offsets) != 1 {
		t.Errorf("offsets = %v, want one page before the cancellation", p.offsets)
	}
}

func TestPaginate_staleTotal(t *testing.T) {
	// The total claims more items than there are: an empty page ends it.
	p := &pager{items: []int{1, 2, 3}, total: 10, failAt: -1}
	got, err := collect(t, kibana.Paginate(context.Background(), 0, 2, p.fetch))
	if err != nil || len(got) != 3 {
		t.Errorf("items = %v, error = %v; want 3 items", got, err)
	}
	if want := []int{0, 2, 3}; !reflect.DeepEqual(p.offsets, want) {
		t.Errorf("offsets = %v, want %v", p.offsets, want)
	}
}
