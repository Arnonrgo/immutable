package immutable

import "iter"

// BatchListBuilder builds a List from a sequence of appends. Values are written
// straight into the 32-wide leaf nodes of the final trie, and List only joins
// the leaves into branches, so Append is a plain array store, nothing is copied
// twice and nothing is held in a second buffer. It is faster than ListBuilder,
// which copies a path through the trie for every value, and allocates about the
// same.
//
// A BatchListBuilder is not safe for concurrent use. After List is called the
// builder is invalid until Reset.
type BatchListBuilder[T any] struct {
	// Completed leaves, all full. leaves plus tail hold the values in order.
	leaves []listNode[T]
	// Leaf being filled, holding tailN values. tail is nil until the first
	// value. A full tail is only moved to leaves when the next value arrives,
	// so tail is non-nil whenever the builder holds at least one value.
	tail  *listLeafNode[T]
	tailN int
	done  bool // set by List; cleared by Reset
}

// NewBatchListBuilder returns a new batch list builder. batchSize is a hint for
// how many values will be appended, used to size the internal bookkeeping up
// front; the builder grows as needed, so it is never a limit. A value of zero
// or less selects a default of 32.
func NewBatchListBuilder[T any](batchSize int) *BatchListBuilder[T] {
	if batchSize <= 0 {
		batchSize = 32 // default hint
	}
	return &BatchListBuilder[T]{leaves: make([]listNode[T], 0, batchSize/listNodeSize)}
}

// Append adds a single value to the end of the list being built.
func (b *BatchListBuilder[T]) Append(value T) {
	assert(!b.done, "immutable.BatchListBuilder: builder invalid after List() invocation")
	if b.tailN == listNodeSize {
		b.sealTail()
	}
	if b.tail == nil {
		b.tail = &listLeafNode[T]{}
	}
	b.tail.children[b.tailN] = value
	b.tailN++
}

// AppendSlice adds all values, in order, to the end of the list being built.
// The values are copied, so the caller may reuse the slice afterwards.
func (b *BatchListBuilder[T]) AppendSlice(values []T) {
	assert(!b.done, "immutable.BatchListBuilder: builder invalid after List() invocation")
	for len(values) > 0 {
		if b.tailN == listNodeSize {
			b.sealTail()
		}
		if b.tail == nil {
			b.tail = &listLeafNode[T]{}
		}
		n := copy(b.tail.children[b.tailN:], values)
		b.tailN += n
		values = values[n:]
	}
}

// sealTail moves the full tail leaf to the completed leaves.
func (b *BatchListBuilder[T]) sealTail() {
	b.tail.occupied = ^uint32(0)
	b.leaves = append(b.leaves, b.tail)
	b.tail, b.tailN = nil, 0
}

// Flush does nothing. Values are no longer committed in batches; the list is
// built once, by List.
//
// Deprecated: Flush is a no-op and is kept only so existing callers compile.
func (b *BatchListBuilder[T]) Flush() {}

// Reset discards all values and makes the builder valid again.
func (b *BatchListBuilder[T]) Reset() {
	b.discard()
	b.done = false
}

// discard drops every node the builder holds, keeping the leaves slice capacity.
func (b *BatchListBuilder[T]) discard() {
	clear(b.leaves) // drop references so the nodes can be collected
	b.leaves = b.leaves[:0]
	b.tail, b.tailN = nil, 0
}

// List builds and returns the list and invalidates the builder. The leaves move
// into the list, so building is O(n/32) and copies no values; call Reset to
// start a new list with the same builder.
func (b *BatchListBuilder[T]) List() *List[T] {
	assert(!b.done, "immutable.BatchListBuilder: builder invalid after List() invocation")
	var list *List[T]
	if len(b.leaves) == 0 {
		// Up to one leaf of values: use the slice representation, as NewList
		// and Append do for lists up to listSliceThreshold.
		elements := make([]T, b.tailN)
		if b.tail != nil {
			copy(elements, b.tail.children[:b.tailN])
		}
		list = &List[T]{root: &listSliceNode[T]{elements: elements}, size: b.tailN}
	} else {
		b.tail.occupied = uint32(1)<<b.tailN - 1
		size := len(b.leaves)*listNodeSize + b.tailN
		b.leaves = append(b.leaves, b.tail)
		list = &List[T]{root: wireListLeaves(b.leaves), size: size}
	}
	b.discard()
	b.done = true
	return list
}

// Len returns the number of values collected so far.
func (b *BatchListBuilder[T]) Len() int {
	assert(!b.done, "immutable.BatchListBuilder: builder invalid after List() invocation")
	return len(b.leaves)*listNodeSize + b.tailN
}

// BatchMapBuilder builds a Map from a sequence of Set calls. It buffers the
// entries and builds the whole trie at once in Map(), which is faster than
// MapBuilder, which walks the trie for every key, and allocates less: no node
// is grown or rebuilt along the way. The result is the same map that calling
// Set for each entry in order would give, the last value for a key wins.
type BatchMapBuilder[K comparable, V any] struct {
	hasher  Hasher[K]
	entries []mapEntry[K, V]
	done    bool
}

// NewBatchMapBuilder returns a new batch-optimized map builder. If hasher is
// nil, a default hasher is chosen from the first key, as in NewMap. batchSize
// is only a hint for how many entries to make room for up front; the builder
// grows past it as needed. A value of zero or less means 32.
func NewBatchMapBuilder[K comparable, V any](hasher Hasher[K], batchSize int) *BatchMapBuilder[K, V] {
	if batchSize <= 0 {
		batchSize = 32
	}
	return &BatchMapBuilder[K, V]{
		hasher:  hasher,
		entries: make([]mapEntry[K, V], 0, batchSize),
	}
}

// Set adds a key/value pair. Setting a key again replaces its value.
func (b *BatchMapBuilder[K, V]) Set(key K, value V) {
	assert(!b.done, "immutable.BatchMapBuilder: builder invalid after Map() invocation")
	if b.hasher == nil {
		b.hasher = NewHasher(key)
	}
	b.entries = appendDoubling(b.entries, mapEntry[K, V]{key: key, value: value})
}

// appendDoubling appends e to s, doubling the capacity when it is full. append
// grows large slices by only about 1.25x, which leaves about four times the
// final size behind as garbage while a batch builder collects its entries.
func appendDoubling[T any](s []T, e T) []T {
	if len(s) == cap(s) {
		grown := make([]T, len(s), max(2*len(s), 8))
		copy(grown, s)
		s = grown
	}
	return append(s, e)
}

// SetMap adds all entries from a regular Go map.
func (b *BatchMapBuilder[K, V]) SetMap(entries map[K]V) {
	for k, v := range entries {
		b.Set(k, v)
	}
}

// Flush does nothing. The map is built once, by Map().
//
// Deprecated: Flush has no effect and is kept only so existing callers compile.
func (b *BatchMapBuilder[K, V]) Flush() {}

// Reset discards all entries so the builder can be reused, including after
// Map(). The hasher is kept.
func (b *BatchMapBuilder[K, V]) Reset() {
	clear(b.entries)
	b.entries = b.entries[:0]
	b.done = false
}

// Map returns the final map and invalidates the builder: Set, Len and a second
// call to Map panic until Reset is called.
func (b *BatchMapBuilder[K, V]) Map() *Map[K, V] {
	assert(!b.done, "immutable.BatchMapBuilder.Map(): duplicate call to fetch map")
	root, size := buildMapRoot(b.entries, b.hasher)
	m := &Map[K, V]{size: size, root: root, hasher: b.hasher}
	clear(b.entries)
	b.entries = nil
	b.done = true
	return m
}

// Len returns the number of Set calls so far. A key that was set more than
// once counts each time, so this is an upper bound on the length of the map;
// the exact length is known once Map() has built it.
func (b *BatchMapBuilder[K, V]) Len() int {
	assert(!b.done, "immutable.BatchMapBuilder: builder invalid after Map() invocation")
	return len(b.entries)
}

// StreamingListBuilder is a BatchListBuilder with methods that take their
// values from a channel, or filter or transform them on the way in. It adds no
// state of its own, so everything said about BatchListBuilder applies.
type StreamingListBuilder[T any] struct {
	*BatchListBuilder[T]
}

// NewStreamingListBuilder creates a streaming list builder. batchSize is a hint
// for how many values will be appended, as for NewBatchListBuilder.
func NewStreamingListBuilder[T any](batchSize int) *StreamingListBuilder[T] {
	return &StreamingListBuilder[T]{BatchListBuilder: NewBatchListBuilder[T](batchSize)}
}

// Stream appends every value received from the channel until it is closed.
func (b *StreamingListBuilder[T]) Stream(values <-chan T) {
	for value := range values {
		b.Append(value)
	}
}

// Filter processes values through a filter function before adding.
func (b *StreamingListBuilder[T]) Filter(values []T, filterFn func(T) bool) {
	for _, value := range values {
		if filterFn(value) {
			b.Append(value)
		}
	}
}

// Transform processes values through a transformation function.
func (b *StreamingListBuilder[T]) Transform(values []T, transformFn func(T) T) {
	for _, value := range values {
		b.Append(transformFn(value))
	}
}

// SortedBatchBuilder builds a SortedMap from a sequence of Set calls. It buffers
// the entries and builds the whole B+tree at once in SortedMap(): the entries
// are sorted with the map's Comparer (not at all when they arrive in strictly
// increasing order) and then cut into leaves and branches, so every node is
// allocated once at its final size. The result holds the same entries that
// calling Set for each one in order would give; the last value for a key wins.
type SortedBatchBuilder[K, V any] struct {
	comparer Comparer[K]
	entries  []sortedBuildEntry[K, V]
	done     bool
}

// NewSortedBatchBuilder returns a new batch builder for sorted maps. If comparer
// is nil, a default comparer is chosen from the first key, as in NewSortedMap.
// batchSize is only a hint for how many entries to make room for up front; the
// builder grows past it as needed. A value of zero or less means 32.
func NewSortedBatchBuilder[K, V any](comparer Comparer[K], batchSize int) *SortedBatchBuilder[K, V] {
	if batchSize <= 0 {
		batchSize = 32
	}
	return &SortedBatchBuilder[K, V]{
		comparer: comparer,
		entries:  make([]sortedBuildEntry[K, V], 0, batchSize),
	}
}

// Set adds a key/value pair. Setting a key again replaces its value.
func (b *SortedBatchBuilder[K, V]) Set(key K, value V) {
	assert(!b.done, "immutable.SortedBatchBuilder: builder invalid after SortedMap() invocation")
	if b.comparer == nil {
		b.comparer = NewComparer(key)
	}
	b.entries = appendDoubling(b.entries, sortedBuildEntry[K, V]{key: key, value: value, seq: uint32(len(b.entries))})
}

// Flush does nothing. The map is built once, by SortedMap().
//
// Deprecated: Flush has no effect and is kept only so existing callers compile.
func (b *SortedBatchBuilder[K, V]) Flush() {}

// Reset discards all entries so the builder can be reused, including after
// SortedMap(). The comparer is kept.
func (b *SortedBatchBuilder[K, V]) Reset() {
	clear(b.entries)
	b.entries = b.entries[:0]
	b.done = false
}

// SortedMap returns the final sorted map and invalidates the builder: Set, Len
// and a second call to SortedMap panic until Reset is called.
func (b *SortedBatchBuilder[K, V]) SortedMap() *SortedMap[K, V] {
	assert(!b.done, "immutable.SortedBatchBuilder.SortedMap(): duplicate call to fetch sorted map")
	root, size := buildSortedMapRoot(b.entries, b.comparer)
	sm := &SortedMap[K, V]{size: size, root: root, comparer: b.comparer}
	clear(b.entries)
	b.entries = nil
	b.done = true
	return sm
}

// Len returns the number of Set calls so far. A key that was set more than once
// counts each time, so this is an upper bound on the length of the map; the
// exact length is known once SortedMap() has built it.
func (b *SortedBatchBuilder[K, V]) Len() int {
	assert(!b.done, "immutable.SortedBatchBuilder: builder invalid after SortedMap() invocation")
	return len(b.entries)
}

// BatchSetBuilder provides enhanced batch operations for efficient Set construction.
type BatchSetBuilder[T comparable] struct {
	mapBuilder *BatchMapBuilder[T, struct{}]
}

// NewBatchSetBuilder returns a new batch-optimized set builder.
func NewBatchSetBuilder[T comparable](hasher Hasher[T], batchSize int) *BatchSetBuilder[T] {
	return &BatchSetBuilder[T]{
		mapBuilder: NewBatchMapBuilder[T, struct{}](hasher, batchSize),
	}
}

// Add inserts a value into the batch buffer.
func (b *BatchSetBuilder[T]) Add(value T) {
	b.mapBuilder.Set(value, struct{}{})
}

// AddSlice adds multiple values efficiently.
func (b *BatchSetBuilder[T]) AddSlice(values []T) {
	for _, value := range values {
		b.Add(value)
	}
}

// Flush does nothing. The set is built once, by Set().
//
// Deprecated: Flush has no effect and is kept only so existing callers compile.
func (b *BatchSetBuilder[T]) Flush() {}

// Set returns the final set and invalidates the builder: Add, Len and a second
// call to Set panic afterwards.
func (b *BatchSetBuilder[T]) Set() *Set[T] {
	return &Set[T]{m: b.mapBuilder.Map()}
}

// Len returns the number of Add calls so far. A value that was added more than
// once counts each time, so this is an upper bound on the size of the set.
func (b *BatchSetBuilder[T]) Len() int {
	return b.mapBuilder.Len()
}

// BatchSortedSetBuilder builds a SortedSet from a sequence of Add calls, with
// the speed of SortedBatchBuilder.
type BatchSortedSetBuilder[T any] struct {
	sortedBuilder *SortedBatchBuilder[T, struct{}]
}

// NewBatchSortedSetBuilder returns a new batch-optimized sorted set builder.
// comparer and batchSize are passed to NewSortedBatchBuilder.
func NewBatchSortedSetBuilder[T any](comparer Comparer[T], batchSize int) *BatchSortedSetBuilder[T] {
	return &BatchSortedSetBuilder[T]{
		sortedBuilder: NewSortedBatchBuilder[T, struct{}](comparer, batchSize),
	}
}

// Add inserts a value into the set being built.
func (b *BatchSortedSetBuilder[T]) Add(value T) {
	b.sortedBuilder.Set(value, struct{}{})
}

// AddSlice adds multiple values efficiently.
func (b *BatchSortedSetBuilder[T]) AddSlice(values []T) {
	for _, value := range values {
		b.Add(value)
	}
}

// Flush does nothing. The set is built once, by SortedSet().
//
// Deprecated: Flush has no effect and is kept only so existing callers compile.
func (b *BatchSortedSetBuilder[T]) Flush() {}

// SortedSet returns the final sorted set and invalidates the builder: Add, Len
// and a second call to SortedSet panic afterwards.
func (b *BatchSortedSetBuilder[T]) SortedSet() *SortedSet[T] {
	return &SortedSet[T]{m: b.sortedBuilder.SortedMap()}
}

// Len returns the number of Add calls so far. A value that was added more than
// once counts each time, so this is an upper bound on the size of the set.
func (b *BatchSortedSetBuilder[T]) Len() int {
	return b.sortedBuilder.Len()
}

// StreamingMapBuilder is a BatchMapBuilder with methods that take their entries
// from an iterator, or filter or transform them on the way in. It adds no state
// of its own, so everything said about BatchMapBuilder applies.
type StreamingMapBuilder[K comparable, V any] struct {
	*BatchMapBuilder[K, V]
}

// NewStreamingMapBuilder creates a streaming map builder. hasher and batchSize
// are passed to NewBatchMapBuilder.
func NewStreamingMapBuilder[K comparable, V any](hasher Hasher[K], batchSize int) *StreamingMapBuilder[K, V] {
	return &StreamingMapBuilder[K, V]{BatchMapBuilder: NewBatchMapBuilder[K, V](hasher, batchSize)}
}

// Stream adds every key/value pair from the iterator.
// Use maps.All(m) to stream from a Go map, or provide any iter.Seq2 source.
func (b *StreamingMapBuilder[K, V]) Stream(seq iter.Seq2[K, V]) {
	for key, value := range seq {
		b.Set(key, value)
	}
}

// Filter processes entries from an iterator, adding only those accepted by filterFn.
func (b *StreamingMapBuilder[K, V]) Filter(seq iter.Seq2[K, V], filterFn func(K, V) bool) {
	for key, value := range seq {
		if filterFn(key, value) {
			b.Set(key, value)
		}
	}
}

// Transform processes entries from an iterator through a transformation function.
func (b *StreamingMapBuilder[K, V]) Transform(seq iter.Seq2[K, V], transformFn func(K, V) (K, V)) {
	for key, value := range seq {
		newKey, newValue := transformFn(key, value)
		b.Set(newKey, newValue)
	}
}

// SetMany adds all entries from a Go map.
//
// Deprecated: SetMany is the same as SetMap and is kept only so existing callers
// compile.
func (b *StreamingMapBuilder[K, V]) SetMany(entries map[K]V) {
	b.SetMap(entries)
}
