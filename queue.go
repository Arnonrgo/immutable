package immutable

// Queue is an immutable FIFO queue implemented using the classic Okasaki
// two-list representation. Elements are dequeued from the front list and
// enqueued onto the back list. When the front becomes empty and the back
// is non-empty, the back is reversed and becomes the new front in O(n),
// preserving amortized O(1) enqueue/dequeue.
//
// Queue is safe for concurrent read access across goroutines.
type Queue[T any] struct {
	front *List[T]
	back  *List[T]
	size  int
}

// NewQueue returns a new queue containing the provided values in order.
// Values are placed on the front for optimal subsequent dequeues.
func NewQueue[T any](values ...T) *Queue[T] {
	if len(values) == 0 {
		return &Queue[T]{front: NewList[T](), back: NewList[T](), size: 0}
	}
	return &Queue[T]{front: NewList(values...), back: NewList[T](), size: len(values)}
}

// NewQueueOf returns a new queue containing the provided slice of values in order.
func NewQueueOf[T any](values []T) *Queue[T] {
	if len(values) == 0 {
		return &Queue[T]{front: NewList[T](), back: NewList[T](), size: 0}
	}
	// Copy values to avoid aliasing.
	buf := make([]T, len(values))
	copy(buf, values)
	return &Queue[T]{front: NewList(buf...), back: NewList[T](), size: len(buf)}
}

// Len returns the total number of elements in the queue.
func (q *Queue[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.size
}

// Empty returns true if the queue is empty.
func (q *Queue[T]) Empty() bool {
	return q == nil || q.size == 0
}

// Peek returns the value at the front of the queue, if any.
// This operation does not modify the queue.
func (q *Queue[T]) Peek() (value T, ok bool) {
	var zero T
	if q == nil || q.size == 0 {
		return zero, false
	}
	// Ensure front has elements to maintain FIFO semantics.
	norm := q.normalize()
	if norm.front != nil && norm.front.Len() > 0 {
		return norm.front.Get(0), true
	}
	return zero, false
}

// Enqueue returns a new queue with v appended to the end.
func (q *Queue[T]) Enqueue(v T) *Queue[T] {
	if q == nil {
		return &Queue[T]{front: NewList[T](), back: NewList(v), size: 1}
	}
	// Prepend onto back so that reversing back produces FIFO order.
	return &Queue[T]{
		front: q.front,
		back:  q.back.Prepend(v),
		size:  q.size + 1,
	}
}

// Dequeue returns a new queue with the first value removed and the value.
// If the queue is empty, ok is false and next is nil.
func (q *Queue[T]) Dequeue() (next *Queue[T], value T, ok bool) {
	var zero T
	if q == nil || q.size == 0 {
		return nil, zero, false
	}

	// If front has elements, pop from there.
	if q.front != nil && q.front.Len() > 0 {
		v := q.front.Get(0)
		newFront := q.front.Slice(1, q.front.Len())
		nextQ := &Queue[T]{front: newFront, back: q.back, size: q.size - 1}
		nextQ = nextQ.normalize()
		return nextQ, v, true
	}

	// Front is empty; if back has elements, reverse it to become the new front.
	if q.back != nil && q.back.Len() > 0 {
		// After normalization, front will be non-empty.
		norm := q.normalize()
		v := norm.front.Get(0)
		newFront := norm.front.Slice(1, norm.front.Len())
		nextQ := &Queue[T]{front: newFront, back: norm.back, size: q.size - 1}
		return nextQ, v, true
	}

	return nil, zero, false
}

// Iterator returns a new iterator over the queue from front to back.
func (q *Queue[T]) Iterator() *QueueIterator[T] {
	itr := &QueueIterator[T]{q: q}
	itr.First()
	return itr
}

// normalize ensures that if the queue is non-empty then the front list is non-empty.
// It returns q if already normalized; otherwise returns a new normalized queue.
func (q *Queue[T]) normalize() *Queue[T] {
	if q == nil || q.size == 0 {
		return q
	}
	if q.front != nil && q.front.Len() > 0 {
		return q
	}
	if q.back == nil || q.back.Len() == 0 {
		return q
	}
	// Reverse back to front, clear back.
	return &Queue[T]{
		front: reverseList(q.back),
		back:  NewList[T](),
		size:  q.size,
	}
}

// reverseList returns a new list containing the elements of l in reverse order.
func reverseList[T any](l *List[T]) *List[T] {
	if l == nil || l.Len() == 0 {
		return NewList[T]()
	}
	// Populate a temporary slice in reverse order, then build a list from it.
	vals := make([]T, l.Len())
	idx := l.Len() - 1
	itr := l.Iterator()
	for !itr.Done() {
		_, v := itr.Next()
		vals[idx] = v
		idx--
	}
	return NewList(vals...)
}

// QueueIterator iterates over a Queue from front to back.
// It first iterates the front list from index 0..n-1, then iterates the
// back list in reverse (since enqueue pushes onto the logical end).
type QueueIterator[T any] struct {
	q        *Queue[T]
	stage    int // 0 = front, 1 = back-reversed, -1 = done
	frontIdx int
	backIdx  int
	index    int // overall index reported to consumers
}

// Done returns true if no more elements remain in the iterator.
func (itr *QueueIterator[T]) Done() bool {
	return itr.stage == -1
}

// First positions the iterator at the first element.
func (itr *QueueIterator[T]) First() {
	if itr.q == nil || itr.q.size == 0 {
		itr.stage = -1
		return
	}

	// Start at front if available; otherwise use back reversed.
	if itr.q.front != nil && itr.q.front.Len() > 0 {
		itr.stage = 0
		itr.frontIdx = 0
		itr.backIdx = -1
		itr.index = 0
		return
	}
	if itr.q.back != nil && itr.q.back.Len() > 0 {
		itr.stage = 1
		itr.frontIdx = -1
		itr.backIdx = itr.q.back.Len() - 1
		itr.index = 0
		return
	}
	itr.stage = -1
}

// Next returns the current index and value and moves the iterator forward.
// ok is false if iteration is complete.
func (itr *QueueIterator[T]) Next() (index int, value T, ok bool) {
	var zero T
	if itr.Done() {
		return -1, zero, false
	}

	switch itr.stage {
	case 0: // front
		v := itr.q.front.Get(itr.frontIdx)
		idx := itr.index
		itr.frontIdx++
		itr.index++
		if itr.frontIdx >= itr.q.front.Len() {
			// move to back-reversed if any
			if itr.q.back != nil && itr.q.back.Len() > 0 {
				itr.stage = 1
				itr.backIdx = itr.q.back.Len() - 1
			} else {
				itr.stage = -1
			}
		}
		return idx, v, true

	case 1: // back-reversed
		v := itr.q.back.Get(itr.backIdx)
		idx := itr.index
		itr.backIdx--
		itr.index++
		if itr.backIdx < 0 {
			itr.stage = -1
		}
		return idx, v, true
	}

	return -1, zero, false
}

// batched enqueues. After calling Queue(), the builder becomes invalid.
type QueueBuilder[T any] struct {
	q *Queue[T]
}

// NewQueueBuilder returns a new builder with an empty queue.
func NewQueueBuilder[T any]() *QueueBuilder[T] {
	return &QueueBuilder[T]{q: NewQueue[T]()}
}

// Enqueue appends a single value to the end of the queue.
func (b *QueueBuilder[T]) Enqueue(v T) {
	assert(b.q != nil, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	b.q = b.q.Enqueue(v)
}

// EnqueueSlice appends all values in order.
func (b *QueueBuilder[T]) EnqueueSlice(values []T) {
	assert(b.q != nil, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	for i := range values {
		b.q = b.q.Enqueue(values[i])
	}
}

// Len returns the current number of elements in the underlying queue.
func (b *QueueBuilder[T]) Len() int {
	assert(b.q != nil, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	return b.q.Len()
}

// Queue returns the built queue and invalidates the builder.
func (b *QueueBuilder[T]) Queue() *Queue[T] {
	assert(b.q != nil, "immutable.QueueBuilder.Queue(): duplicate call to fetch queue")
	q := b.q
	b.q = nil
	return q
}
