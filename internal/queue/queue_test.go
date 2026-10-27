package queue

import (
	"slices"
	"testing"
)

func TestUnboundedKeepsOrderAndCoalescesSignals(t *testing.T) {
	q := New[int]()
	for i := range 1000 {
		q.Push(i)
	}

	select {
	case <-q.Ready():
	default:
		t.Fatal("expected a ready signal after pushing")
	}

	got := q.Drain()
	want := make([]int, 1000)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(got, want) {
		t.Fatalf("drain returned %d items out of order", len(got))
	}

	select {
	case <-q.Ready():
		t.Fatal("pushes should collapse into a single ready signal")
	default:
	}
	if items := q.Drain(); len(items) != 0 {
		t.Fatalf("expected empty queue, got %v", items)
	}
}

func TestCloseHandsBackWhatIsLeftAndRefusesMore(t *testing.T) {
	q := New[int]()
	q.Push(1)
	q.Push(2)
	if got := q.Close(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("close returned %v, want [1 2]", got)
	}
	if q.Push(3) {
		t.Fatal("a closed queue should refuse pushes")
	}
	if got := q.Drain(); len(got) != 0 {
		t.Fatalf("nothing should be queued after close, got %v", got)
	}
}
