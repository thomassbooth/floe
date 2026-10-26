// Package queue provides an unbounded FIFO queue used to hand work between
// goroutines without ever blocking the producer.
package queue

import "sync"

// Unbounded is a FIFO queue whose Push never blocks. A consumer waits on
// Ready and then takes everything queued so far with Drain.
type Unbounded[T any] struct {
	mu    sync.Mutex
	items []T
	ready chan struct{}
}

// New returns an empty queue.
func New[T any]() *Unbounded[T] {
	return &Unbounded[T]{ready: make(chan struct{}, 1)}
}

// Push appends v to the queue and wakes the consumer.
func (q *Unbounded[T]) Push(v T) {
	q.mu.Lock()
	q.items = append(q.items, v)
	q.mu.Unlock()

	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// Ready receives a value whenever the queue may have items. Several pushes
// can collapse into a single signal, so always Drain after receiving.
func (q *Unbounded[T]) Ready() <-chan struct{} {
	return q.ready
}

// Drain removes and returns every queued item in FIFO order.
func (q *Unbounded[T]) Drain() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.items
	q.items = nil
	return items
}
