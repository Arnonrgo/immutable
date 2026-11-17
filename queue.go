package immutable

import "sync/atomic"

// Queue is an immutable FIFO queue implemented using the classic Okasaki
// two-list representation backed by cons-lists. Elements are enqueued onto
// the back list and dequeued from the front list. When the front is empty and
// the back is not, Peek, Dequeue and Iterator turn the back list around in O(n)
// to obtain the front.
//
// Complexity: Enqueue is O(1). Peek and Dequeue are O(1) once the front is
// populated, and a queue that has only a back list (one made by an Enqueue loop,
// or whose front was used up) pays one O(n) reversal the first time it is read.
// The result is kept with the queue, so reading the same queue value again, or
// branching several queues from it, does not repeat the reversal; the cost is
// that a queue that has been read this way holds a second copy of its list for as
// long as the queue itself is alive. In the usual linear pattern
// (q = q.Enqueue(x), q, v, _ = q.Dequeue()) that makes Dequeue amortized O(1).
// Queues built with NewQueue, NewQueueOf or QueueBuilder start with a populated
// front and do not pay for a reversal until the front is used up.
//
// Queue is safe for concurrent use by multiple goroutines, because a queue is
// never modified once it is returned. The one thing that changes is the
// reversed copy kept for a queue whose front is empty; it is stored atomically,
// so concurrent readers either share it or each make an equal one.
type Queue[T any] struct {
	front *consList[T] // oldest element first
	back  *consList[T] // newest element first
	// norm is the queue with the same elements and a populated front. It is
	// set the first time a queue that has a back list and no front is read, and
	// is not used otherwise. The length of the queue is the sum of the lengths of
	// the two lists, so there is no separate size to keep in step with them.
	norm atomic.Pointer[Queue[T]]
}

// NewQueue returns a new queue containing the provided values in order.
// Values are placed on the front for optimal subsequent dequeues.
func NewQueue[T any](values ...T) *Queue[T] {
	return &Queue[T]{front: consFromSlice(values)}
}

// NewQueueOf returns a new queue containing the provided slice of values in order.
// The queue does not refer to the slice, so the caller may change it afterwards.
func NewQueueOf[T any](values []T) *Queue[T] {
	return &Queue[T]{front: consFromSlice(values)}
}

// Len returns the total number of elements in the queue.
func (q *Queue[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.front.length() + q.back.length()
}

// Empty returns true if the queue is empty.
func (q *Queue[T]) Empty() bool {
	return q == nil || (q.front == nil && q.back == nil)
}

// Peek returns the value at the front of the queue, if any.
// This operation does not modify the queue.
func (q *Queue[T]) Peek() (value T, ok bool) {
	if q.Empty() {
		return value, false
	}
	return q.normalize().front.head, true
}

// Enqueue returns a new queue with v appended to the end.
func (q *Queue[T]) Enqueue(v T) *Queue[T] {
	if q == nil {
		return &Queue[T]{back: cons(v, nil)}
	}
	return &Queue[T]{front: q.front, back: cons(v, q.back)}
}

// Dequeue returns a new queue with the first value removed and the value.
// If the queue is empty, ok is false and next is nil.
func (q *Queue[T]) Dequeue() (next *Queue[T], value T, ok bool) {
	if q.Empty() {
		return nil, value, false
	}
	n := q.normalize()
	return &Queue[T]{front: n.front.tail, back: n.back}, n.front.head, true
}

// Iterator returns a new iterator over the queue from front to back.
func (q *Queue[T]) Iterator() *QueueIterator[T] {
	if q == nil {
		return &QueueIterator[T]{}
	}
	n := q.normalize()
	return &QueueIterator[T]{node: n.front, back: n.back}
}

// normalize returns a queue with the same elements whose front is populated
// whenever the queue is not empty. It returns q itself when q already is one.
// Otherwise q has a back list and no front, and the result is q's back list
// turned around, which is made once and kept in q.norm.
func (q *Queue[T]) normalize() *Queue[T] {
	if q.front != nil || q.back == nil {
		return q
	}
	if n := q.norm.Load(); n != nil {
		return n
	}
	n := &Queue[T]{front: q.back.reverse()}
	if !q.norm.CompareAndSwap(nil, n) {
		n = q.norm.Load() // another goroutine got there first; share its result
	}
	return n
}

// QueueIterator iterates over a Queue from front to back.
type QueueIterator[T any] struct {
	node  *consList[T] // the next element, or nil once the front part is used up
	back  *consList[T] // newest first; turned around only if the walk reaches it
	index int
}

// Done returns true if no more elements remain in the iterator.
func (itr *QueueIterator[T]) Done() bool {
	return itr.node == nil && itr.back == nil
}

// First positions the iterator at the first element.
func (itr *QueueIterator[T]) First() {
	// no-op: iterator is positioned at first element on creation
}

// Next returns the current index and value and moves the iterator forward.
// ok is false if iteration is complete.
func (itr *QueueIterator[T]) Next() (index int, value T, ok bool) {
	if itr.node == nil && itr.back != nil {
		// The front is used up: turn the back list around and walk that.
		itr.node, itr.back = itr.back.reverse(), nil
	}
	if itr.node == nil {
		return -1, value, false
	}
	value = itr.node.head
	itr.node = itr.node.tail
	index = itr.index
	itr.index++
	return index, value, true
}

// QueueBuilder supports batched enqueues. After calling Queue(), the builder
// becomes invalid: every method, including a second Queue(), panics.
//
// The values are collected in a slice and the queue is made once, by Queue(),
// with all of them in its front list, so taking elements out of the built queue
// never has to turn a list around.
//
// The zero value is an empty builder that is ready to use.
type QueueBuilder[T any] struct {
	values []T
	done   bool // Queue() has been called
}

// NewQueueBuilder returns a new builder with an empty queue.
func NewQueueBuilder[T any]() *QueueBuilder[T] {
	return &QueueBuilder[T]{}
}

// Enqueue appends a single value to the end of the queue.
func (b *QueueBuilder[T]) Enqueue(v T) {
	assert(!b.done, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	b.values = appendDoubling(b.values, v)
}

// EnqueueSlice appends all values in order. The builder keeps its own copy, so
// the caller may change the slice afterwards.
func (b *QueueBuilder[T]) EnqueueSlice(values []T) {
	assert(!b.done, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	if need := len(b.values) + len(values); need > cap(b.values) {
		grown := make([]T, len(b.values), max(2*cap(b.values), need))
		copy(grown, b.values)
		b.values = grown
	}
	b.values = append(b.values, values...)
}

// Len returns the current number of elements in the underlying queue.
func (b *QueueBuilder[T]) Len() int {
	assert(!b.done, "immutable.QueueBuilder: builder invalid after Queue() invocation")
	return len(b.values)
}

// Queue returns the built queue and invalidates the builder.
func (b *QueueBuilder[T]) Queue() *Queue[T] {
	assert(!b.done, "immutable.QueueBuilder.Queue(): duplicate call to fetch queue")
	q := &Queue[T]{front: consFromSlice(b.values)}
	b.values, b.done = nil, true
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

// length returns the number of elements; a nil list has none. O(1).
func (l *consList[T]) length() int {
	if l == nil {
		return 0
	}
	return l.len
}

// reverse returns a new cons-list with elements in reverse order. O(n).
func (l *consList[T]) reverse() *consList[T] {
	var out *consList[T]
	for node := l; node != nil; node = node.tail {
		out = cons(node.head, out)
	}
	return out
}

// consFromSlice builds a cons-list from a slice, preserving order. O(n).
func consFromSlice[T any](values []T) *consList[T] {
	var list *consList[T]
	for i := len(values) - 1; i >= 0; i-- {
		list = cons(values[i], list)
	}
	return list
}
