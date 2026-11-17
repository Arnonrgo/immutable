package immutable

// Queue is an immutable FIFO queue implemented using the classic Okasaki
// two-list representation backed by cons-lists. Elements are enqueued onto
// the back list and dequeued from the front list. When the front is empty and
// the back is not, Peek and Dequeue reverse the back list in O(n) to obtain
// the front.
//
// Complexity: Enqueue is O(1). Peek and Dequeue are amortized O(1) when each
// queue version is used once, which is the usual linear pattern
// (q = q.Enqueue(x), q, v, _ = q.Dequeue()). The reversal is not cached, so
// calling Peek or Dequeue repeatedly on the same queue value whose front is
// empty costs O(n) every time (the known limitation of this structure under
// persistent reuse). Queues built with NewQueue or NewQueueOf start with a
// populated front and do not pay this cost until the front is drained.
//
// Queue is safe for concurrent read access across goroutines.
type Queue[T any] struct {
	front *consList[T]
	back  *consList[T]
	size  int
}

// NewQueue returns a new queue containing the provided values in order.
// Values are placed on the front for optimal subsequent dequeues.
func NewQueue[T any](values ...T) *Queue[T] {
	return &Queue[T]{
		front: consFromSlice(values),
		back:  nil,
		size:  len(values),
	}
}

// NewQueueOf returns a new queue containing the provided slice of values in order.
func NewQueueOf[T any](values []T) *Queue[T] {
	buf := make([]T, len(values))
	copy(buf, values)
	return &Queue[T]{
		front: consFromSlice(buf),
		back:  nil,
		size:  len(buf),
	}
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
	norm := q.normalize()
	if norm.front != nil {
		return norm.front.head, true
	}
	return zero, false
}

// Enqueue returns a new queue with v appended to the end.
func (q *Queue[T]) Enqueue(v T) *Queue[T] {
	if q == nil {
		return &Queue[T]{front: nil, back: cons(v, nil), size: 1}
	}
	return &Queue[T]{
		front: q.front,
		back:  cons(v, q.back),
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

	norm := q.normalize()
	v := norm.front.head
	nextQ := &Queue[T]{
		front: norm.front.tail,
		back:  norm.back,
		size:  q.size - 1,
	}
	return nextQ, v, true
}

// Iterator returns a new iterator over the queue from front to back.
func (q *Queue[T]) Iterator() *QueueIterator[T] {
	itr := &QueueIterator[T]{}
	if q == nil || q.size == 0 {
		itr.done = true
		return itr
	}
	itr.frontNode = q.front
	if q.back != nil {
		itr.backSlice = q.back.reverse().toSlice()
	}
	return itr
}

// normalize ensures that if the queue is non-empty then the front list
// is non-empty. Returns q unchanged if already normalized.
func (q *Queue[T]) normalize() *Queue[T] {
	if q.front != nil {
		return q
	}
	if q.back == nil {
		return q
	}
	return &Queue[T]{
		front: q.back.reverse(),
		back:  nil,
		size:  q.size,
	}
}

// QueueIterator iterates over a Queue from front to back.
type QueueIterator[T any] struct {
	frontNode *consList[T]
	backSlice []T
	backIdx   int
	index     int
	done      bool
}

// Done returns true if no more elements remain in the iterator.
func (itr *QueueIterator[T]) Done() bool {
	return itr.done
}

// First positions the iterator at the first element.
func (itr *QueueIterator[T]) First() {
	// no-op: iterator is positioned at first element on creation
}

// Next returns the current index and value and moves the iterator forward.
// ok is false if iteration is complete.
func (itr *QueueIterator[T]) Next() (index int, value T, ok bool) {
	var zero T
	if itr.done {
		return -1, zero, false
	}

	idx := itr.index
	itr.index++

	if itr.frontNode != nil {
		v := itr.frontNode.head
		itr.frontNode = itr.frontNode.tail
		if itr.frontNode == nil && itr.backIdx >= len(itr.backSlice) {
			itr.done = true
		}
		return idx, v, true
	}

	if itr.backIdx < len(itr.backSlice) {
		v := itr.backSlice[itr.backIdx]
		itr.backIdx++
		if itr.backIdx >= len(itr.backSlice) {
			itr.done = true
		}
		return idx, v, true
	}

	itr.done = true
	return -1, zero, false
}

// QueueBuilder supports batched enqueues. After calling Queue(), the builder
// becomes invalid: every method, including a second Queue(), panics.
//
// The zero value is an empty builder that is ready to use.
type QueueBuilder[T any] struct {
	q    *Queue[T] // nil is the empty queue, as everywhere else in this package
	done bool      // Queue() has been called
}

// NewQueueBuilder returns a new builder with an empty queue.
func NewQueueBuilder[T any]() *QueueBuilder[T] {
	return &QueueBuilder[T]{q: NewQueue[T]()}
}

// Enqueue appends a single value to the end of the queue.
func (b *QueueBuilder[T]) Enqueue(v T) {
	assert(!b.done, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	b.q = b.q.Enqueue(v)
}

// EnqueueSlice appends all values in order.
func (b *QueueBuilder[T]) EnqueueSlice(values []T) {
	assert(!b.done, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	for i := range values {
		b.q = b.q.Enqueue(values[i])
	}
}

// Len returns the current number of elements in the underlying queue.
func (b *QueueBuilder[T]) Len() int {
	assert(!b.done, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	return b.q.Len()
}

// Queue returns the built queue and invalidates the builder.
func (b *QueueBuilder[T]) Queue() *Queue[T] {
	assert(!b.done, "immutable.QueueBuilder.Queue(): duplicate call to fetch queue")
	q := b.q
	if q == nil { // zero-value builder that was never used
		q = NewQueue[T]()
	}
	b.q, b.done = nil, true
	return q
}

// consList is an immutable singly-linked list (cons-list).
// Prepend (cons) is O(1) with full structural sharing.
type consList[T any] struct {
	head T
	tail *consList[T]
	len  int
}

// cons prepends v onto list, returning a new list. O(1).
func cons[T any](v T, list *consList[T]) *consList[T] {
	n := 1
	if list != nil {
		n = list.len + 1
	}
	return &consList[T]{head: v, tail: list, len: n}
}

// reverse returns a new cons-list with elements in reverse order. O(n).
func (l *consList[T]) reverse() *consList[T] {
	var out *consList[T]
	for node := l; node != nil; node = node.tail {
		out = cons(node.head, out)
	}
	return out
}

// toSlice returns the elements as a slice in list order. O(n).
func (l *consList[T]) toSlice() []T {
	if l == nil {
		return nil
	}
	s := make([]T, 0, l.len)
	for node := l; node != nil; node = node.tail {
		s = append(s, node.head)
	}
	return s
}

// consFromSlice builds a cons-list from a slice, preserving order. O(n).
func consFromSlice[T any](values []T) *consList[T] {
	var list *consList[T]
	for i := len(values) - 1; i >= 0; i-- {
		list = cons(values[i], list)
	}
	return list
}
