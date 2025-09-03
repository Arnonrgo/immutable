package immutable

import (
	"cmp"
)

// BatchListBuilder provides enhanced batch operations for efficient List construction.
// Optimized for bulk insertions with minimal allocations.
type BatchListBuilder[T any] struct {
	list      *List[T]
	batchSize int
	buffer    []T
}

// NewBatchListBuilder returns a new batch-optimized list builder.
// batchSize determines the internal buffer size for batch operations.
func NewBatchListBuilder[T any](batchSize int) *BatchListBuilder[T] {
	if batchSize <= 0 {
		batchSize = 32 // default batch size
	}
	return &BatchListBuilder[T]{
		list:      NewList[T](),
		batchSize: batchSize,
		buffer:    make([]T, 0, batchSize),
	}
}

// Append adds a single value to the batch buffer.
// Values are flushed to the list when buffer reaches capacity.
func (b *BatchListBuilder[T]) Append(value T) {
	b.buffer = append(b.buffer, value)
	if len(b.buffer) >= b.batchSize {
		b.Flush()
	}
}

// AppendSlice adds multiple values efficiently.
// Automatically handles batching for optimal performance.
func (b *BatchListBuilder[T]) AppendSlice(values []T) {
	for _, value := range values {
		b.Append(value)
	}
}

// Flush commits all buffered values to the underlying list.
func (b *BatchListBuilder[T]) Flush() {
	if len(b.buffer) == 0 {
		return
	}

	// Fast path: if underlying list is slice-backed, extend in one allocation.
	if sliceNode, ok := b.list.root.(*listSliceNode[T]); ok {
		newLen := b.list.size + len(b.buffer)
		newElements := make([]T, newLen)
		copy(newElements, sliceNode.elements)
		copy(newElements[b.list.size:], b.buffer)
		b.list.root = &listSliceNode[T]{elements: newElements}
		b.list.size = newLen
	} else {
		// Fallback: append one-by-one using mutable trie path
		for _, value := range b.buffer {
			b.list = b.list.append(value, true) // mutable for performance
		}
	}

	// Clear buffer (reuse capacity)
	b.buffer = b.buffer[:0]
}

// Reset clears the builder state while retaining buffer capacity.
func (b *BatchListBuilder[T]) Reset() {
	b.list = NewList[T]()
	b.buffer = b.buffer[:0]
}

// List returns the final list and invalidates the builder.
// Automatically flushes any remaining buffered values.
func (b *BatchListBuilder[T]) List() *List[T] {
	b.Flush()
	list := b.list
	b.list = nil
	return list
}

// Len returns the total number of elements (committed + buffered).
func (b *BatchListBuilder[T]) Len() int {
	if b.list == nil {
		return 0
	}
	return b.list.Len() + len(b.buffer)
}

// BatchMapBuilder provides enhanced batch operations for efficient Map construction.
type BatchMapBuilder[K comparable, V any] struct {
	m         *Map[K, V]
	batchSize int
	buffer    []mapEntry[K, V]
}

// NewBatchMapBuilder returns a new batch-optimized map builder.
func NewBatchMapBuilder[K comparable, V any](hasher Hasher[K], batchSize int) *BatchMapBuilder[K, V] {
	if batchSize <= 0 {
		batchSize = 32
	}
	return &BatchMapBuilder[K, V]{
		m:         NewMap[K, V](hasher),
		batchSize: batchSize,
		buffer:    make([]mapEntry[K, V], 0, batchSize),
	}
}

// Set adds a key/value pair to the batch buffer.
func (b *BatchMapBuilder[K, V]) Set(key K, value V) {
	b.buffer = append(b.buffer, mapEntry[K, V]{key: key, value: value})
	if len(b.buffer) >= b.batchSize {
		b.Flush()
	}
}

// SetMap adds all entries from a regular Go map.
func (b *BatchMapBuilder[K, V]) SetMap(entries map[K]V) {
	for k, v := range entries {
		b.Set(k, v)
	}
}

// Flush commits all buffered entries to the underlying map.
func (b *BatchMapBuilder[K, V]) Flush() {
	if len(b.buffer) == 0 {
		return
	}

	// Fast path: if map is empty, build an array node in one shot with last-write-wins semantics.
	if b.m.root == nil {
		var dedup []mapEntry[K, V]
		if len(b.buffer) <= maxArrayMapSize {
			// Tiny buffer: use slice-based last-occurrence dedup without maps.
			for i := len(b.buffer) - 1; i >= 0; i-- {
				key := b.buffer[i].key
				found := false
				for _, e := range dedup {
					if b.m.hasher != nil {
						if b.m.hasher.Equal(e.key, key) {
							found = true
							break
						}
					} else {
						if any(e.key) == any(key) {
							found = true
							break
						}
					}
				}
				if !found {
					dedup = append(dedup, b.buffer[i])
				}
			}
			// Reverse to restore original order of last occurrences
			for i, j := 0, len(dedup)-1; i < j; i, j = i+1, j-1 {
				dedup[i], dedup[j] = dedup[j], dedup[i]
			}
		} else {
			// Larger buffer: map-based dedup
			seen := make(map[K]struct{}, len(b.buffer))
			for i := len(b.buffer) - 1; i >= 0; i-- {
				e := b.buffer[i]
				if _, ok := seen[e.key]; ok {
					continue
				}
				seen[e.key] = struct{}{}
				dedup = append(dedup, e)
			}
			for i, j := 0, len(dedup)-1; i < j; i, j = i+1, j-1 {
				dedup[i], dedup[j] = dedup[j], dedup[i]
			}
		}
		// Ensure hasher is set for Get operations
		if b.m.hasher == nil && len(dedup) > 0 {
			b.m.hasher = NewHasher(dedup[0].key)
		}
		// Install as array node
		b.m.size = len(dedup)
		b.m.root = &mapArrayNode[K, V]{entries: dedup}
	} else if arr, ok := b.m.root.(*mapArrayNode[K, V]); ok {
		// Small-structure fast path: stay in array node if total entries remain under threshold.
		// Build last-write-wins overrides and first-seen order for new keys (slice-based for tiny buffers).
		// Stage last-occurrence per key as slice for tiny buffers; fallback to map for larger buffers.
		var last []mapEntry[K, V]
		if len(b.buffer) <= maxArrayMapSize {
			for i := len(b.buffer) - 1; i >= 0; i-- {
				e := b.buffer[i]
				found := false
				for _, le := range last {
					if b.m.hasher != nil {
						if b.m.hasher.Equal(le.key, e.key) {
							found = true
							break
						}
					} else {
						if any(le.key) == any(e.key) {
							found = true
							break
						}
					}
				}
				if !found {
					last = append(last, e)
				}
			}
			// Reverse to keep first-seen order among last-occurrences
			for i, j := 0, len(last)-1; i < j; i, j = i+1, j-1 {
				last[i], last[j] = last[j], last[i]
			}
		} else {
			seenNew := make(map[K]struct{}, len(b.buffer))
			for _, e := range b.buffer {
				if _, ok := seenNew[e.key]; ok {
					continue
				}
				seenNew[e.key] = struct{}{}
				last = append(last, e)
			}
		}
		// Track original keys
		orig := make(map[K]struct{}, len(arr.entries))
		for _, e := range arr.entries {
			orig[e.key] = struct{}{}
		}
		// Copy existing and apply overrides from last
		newEntries := make([]mapEntry[K, V], len(arr.entries))
		copy(newEntries, arr.entries)
		for i, e := range newEntries {
			for _, le := range last {
				// If key matches, override value
				match := false
				if b.m.hasher != nil {
					match = b.m.hasher.Equal(e.key, le.key)
				} else {
					match = any(e.key) == any(le.key)
				}
				if match {
					newEntries[i] = mapEntry[K, V]{key: e.key, value: le.value}
					break
				}
			}
		}
		// Append truly new keys
		toAppend := make([]mapEntry[K, V], 0)
		for _, le := range last {
			if _, existed := orig[le.key]; !existed {
				toAppend = append(toAppend, le)
			}
		}
		newCount := len(newEntries) + len(toAppend)
		if newCount <= maxArrayMapSize {
			newEntries = append(newEntries, toAppend...)
			b.m.size = newCount
			b.m.root = &mapArrayNode[K, V]{entries: newEntries}
		} else {
			// Fallback: set one-by-one using mutable path
			for _, e := range b.buffer {
				b.m = b.m.set(e.key, e.value, true)
			}
		}
	} else {
		// Fallback: set one-by-one using mutable path
		for _, entry := range b.buffer {
			b.m = b.m.set(entry.key, entry.value, true) // mutable for performance
		}
	}

	// Clear buffer (reuse capacity)
	b.buffer = b.buffer[:0]
}

// Reset clears the builder state while retaining buffer capacity.
func (b *BatchMapBuilder[K, V]) Reset() {
	var hasher Hasher[K]
	if b.m != nil {
		hasher = b.m.hasher
	}
	b.m = NewMap[K, V](hasher)
	b.buffer = b.buffer[:0]
}

// Map returns the final map and invalidates the builder.
func (b *BatchMapBuilder[K, V]) Map() *Map[K, V] {
	b.Flush()
	m := b.m
	b.m = nil
	return m
}

// Len returns the total number of entries (committed + buffered).
func (b *BatchMapBuilder[K, V]) Len() int {
	if b.m == nil {
		return 0
	}
	return b.m.Len() + len(b.buffer)
}

// StreamingListBuilder provides streaming operations with configurable flush triggers.
type StreamingListBuilder[T any] struct {
	*BatchListBuilder[T]
	autoFlushSize    int
	autoFlushEnabled bool
}

// NewStreamingListBuilder creates a builder with automatic flush capabilities.
func NewStreamingListBuilder[T any](batchSize, autoFlushSize int) *StreamingListBuilder[T] {
	return &StreamingListBuilder[T]{
		BatchListBuilder: NewBatchListBuilder[T](batchSize),
		autoFlushSize:    max(autoFlushSize, batchSize),
		autoFlushEnabled: autoFlushSize > 0,
	}
}

// Stream processes values through a streaming pipeline.
// Automatically flushes when size thresholds are reached.
func (b *StreamingListBuilder[T]) Stream(values <-chan T) {
	for value := range values {
		b.Append(value)

		// Auto-flush when reaching threshold
		if b.autoFlushEnabled && b.Len() >= b.autoFlushSize {
			b.Flush()
		}
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

// SortedBatchBuilder provides batch operations optimized for sorted data.
type SortedBatchBuilder[K cmp.Ordered, V any] struct {
	sm        *SortedMap[K, V]
	batchSize int
	buffer    []mapEntry[K, V]
	sorted    bool // whether buffer is kept sorted
}

// NewSortedBatchBuilder creates a batch builder for sorted maps.
// If maintainSort is true, entries are kept sorted in the buffer for optimal insertion.
func NewSortedBatchBuilder[K cmp.Ordered, V any](comparer Comparer[K], batchSize int, maintainSort bool) *SortedBatchBuilder[K, V] {
	if batchSize <= 0 {
		batchSize = 32
	}
	return &SortedBatchBuilder[K, V]{
		sm:        NewSortedMap[K, V](comparer),
		batchSize: batchSize,
		buffer:    make([]mapEntry[K, V], 0, batchSize),
		sorted:    maintainSort,
	}
}

// Set adds a key/value pair, maintaining sort order if enabled.
func (b *SortedBatchBuilder[K, V]) Set(key K, value V) {
	entry := mapEntry[K, V]{key: key, value: value}

	if b.sorted && len(b.buffer) > 0 {
		// Insert in sorted position
		insertIdx := 0
		for i, existing := range b.buffer {
			if defaultCompare(key, existing.key) <= 0 {
				insertIdx = i
				break
			}
			insertIdx = i + 1
		}

		// Insert at correct position
		b.buffer = append(b.buffer, mapEntry[K, V]{})
		copy(b.buffer[insertIdx+1:], b.buffer[insertIdx:])
		b.buffer[insertIdx] = entry
	} else {
		// Simple append
		b.buffer = append(b.buffer, entry)
	}

	if len(b.buffer) >= b.batchSize {
		b.Flush()
	}
}

// Flush commits buffered entries to the sorted map.
func (b *SortedBatchBuilder[K, V]) Flush() {
	if len(b.buffer) == 0 {
		return
	}

	// Batch set all buffered entries
	for _, entry := range b.buffer {
		b.sm = b.sm.set(entry.key, entry.value, true)
	}

	// Clear buffer
	b.buffer = b.buffer[:0]
}

// SortedMap returns the final sorted map.
func (b *SortedBatchBuilder[K, V]) SortedMap() *SortedMap[K, V] {
	b.Flush()
	sm := b.sm
	b.sm = nil
	return sm
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

// Flush commits all buffered values to the underlying set.
func (b *BatchSetBuilder[T]) Flush() {
	b.mapBuilder.Flush()
}

// Set returns the final set and invalidates the builder.
func (b *BatchSetBuilder[T]) Set() *Set[T] {
	m := b.mapBuilder.Map()
	if m == nil {
		return nil
	}
	return &Set[T]{m: m}
}

// Len returns the total number of elements (committed + buffered).
func (b *BatchSetBuilder[T]) Len() int {
	return b.mapBuilder.Len()
}

// BatchSortedSetBuilder provides enhanced batch operations for efficient SortedSet construction.
type BatchSortedSetBuilder[T cmp.Ordered] struct {
	sortedBuilder *SortedBatchBuilder[T, struct{}]
}

// NewBatchSortedSetBuilder returns a new batch-optimized sorted set builder.
func NewBatchSortedSetBuilder[T cmp.Ordered](comparer Comparer[T], batchSize int, maintainSort bool) *BatchSortedSetBuilder[T] {
	return &BatchSortedSetBuilder[T]{
		sortedBuilder: NewSortedBatchBuilder[T, struct{}](comparer, batchSize, maintainSort),
	}
}

// Add inserts a value into the batch buffer, maintaining sort order if enabled.
func (b *BatchSortedSetBuilder[T]) Add(value T) {
	b.sortedBuilder.Set(value, struct{}{})
}

// AddSlice adds multiple values efficiently.
func (b *BatchSortedSetBuilder[T]) AddSlice(values []T) {
	for _, value := range values {
		b.Add(value)
	}
}

// Flush commits buffered values to the sorted set.
func (b *BatchSortedSetBuilder[T]) Flush() {
	b.sortedBuilder.Flush()
}

// SortedSet returns the final sorted set.
func (b *BatchSortedSetBuilder[T]) SortedSet() *SortedSet[T] {
	sm := b.sortedBuilder.SortedMap()
	if sm == nil {
		return nil
	}
	return &SortedSet[T]{m: sm}
}

// Len returns the total number of elements (committed + buffered).
func (b *BatchSortedSetBuilder[T]) Len() int {
	return b.sortedBuilder.sm.Len() + len(b.sortedBuilder.buffer)
}

// StreamingMapBuilder provides streaming operations with configurable flush triggers for Maps.
type StreamingMapBuilder[K comparable, V any] struct {
	*BatchMapBuilder[K, V]
	autoFlushSize    int
	autoFlushEnabled bool
}

// NewStreamingMapBuilder creates a map builder with automatic flush capabilities.
func NewStreamingMapBuilder[K comparable, V any](hasher Hasher[K], batchSize, autoFlushSize int) *StreamingMapBuilder[K, V] {
	return &StreamingMapBuilder[K, V]{
		BatchMapBuilder:  NewBatchMapBuilder[K, V](hasher, batchSize),
		autoFlushSize:    max(autoFlushSize, batchSize),
		autoFlushEnabled: autoFlushSize > 0,
	}
}

// Stream processes key/value pairs through a streaming pipeline.
func (b *StreamingMapBuilder[K, V]) Stream(entries <-chan mapEntry[K, V]) {
	for entry := range entries {
		b.Set(entry.key, entry.value)

		// Auto-flush when reaching threshold
		if b.autoFlushEnabled && b.Len() >= b.autoFlushSize {
			b.Flush()
		}
	}
}

// Filter processes entries through a filter function before adding.
func (b *StreamingMapBuilder[K, V]) Filter(entries []mapEntry[K, V], filterFn func(K, V) bool) {
	for _, entry := range entries {
		if filterFn(entry.key, entry.value) {
			b.Set(entry.key, entry.value)
		}
	}
}

// Transform processes entries through a transformation function.
func (b *StreamingMapBuilder[K, V]) Transform(entries []mapEntry[K, V], transformFn func(K, V) (K, V)) {
	for _, entry := range entries {
		newKey, newValue := transformFn(entry.key, entry.value)
		b.Set(newKey, newValue)
	}
}

// SetMany adds multiple key/value pairs efficiently from a map.
func (b *StreamingMapBuilder[K, V]) SetMany(entries map[K]V) {
	for key, value := range entries {
		b.Set(key, value)

		// Auto-flush when reaching threshold
		if b.autoFlushEnabled && b.Len() >= b.autoFlushSize {
			b.Flush()
		}
	}
}
