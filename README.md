Immutable [![release](https://img.shields.io/github/release/arnonrgo/immutable.svg)](https://pkg.go.dev/github.com/arnonrgo/immutable) ![test](https://github.com/arnonrgo/immutable/workflows/test/badge.svg) ![coverage](https://img.shields.io/codecov/c/github/arnonrgo/immutable/master.svg) ![license](https://img.shields.io/github/license/arnonrgo/immutable.svg)
=========

This repository contains *generic* immutable collection types for Go. It includes
`List`, `Map`, `SortedMap`, `Set` and `SortedSet` implementations. Immutable collections can
provide efficient, lock free sharing of data by requiring that edits to the
collections return new collections.

The collection types in this library are meant to mimic Go built-in collections
such as`slice` and `map`. The primary usage difference between Go collections
and `immutable` collections is that `immutable` collections always return a new
collection on mutation so you will need to save the new reference.

This project is a fork of [github.com/benbjohnson/immutable](https://github.com/benbjohnson/immutable) with additional performance enhancements and builder APIs.

Requires Go 1.26 or later. See [CHANGELOG.md](CHANGELOG.md) for breaking changes since v0.6.0.

**Performance**: Immutability has a cost: every persistent write returns a new
collection and allocates. For bulk construction use the builders (`ListBuilder`,
`MapBuilder`, ...), which mutate in place until you take the result. On the
reference machine (Apple M1 Pro, 100k int keys) `MapBuilder` builds a map in
68-129 ns per element, versus 38 ns for a built-in Go map and 603-849 ns for a
loop of persistent `Set` calls. See the
[Current Performance Snapshot](#current-performance-snapshot) for the numbers
and how they were measured.

Immutable collections are not for every situation, however, as they can incur
additional CPU and memory overhead. Please evaluate the cost/benefit for your
particular project.

Special thanks to the [Immutable.js](https://immutable-js.github.io/immutable-js/)
team as the `List` & `Map` implementations are loose ports from that project.

Forked from https://github.com/benbjohnson/immutable with the following enhancements:

### **Performance Optimizations**

**Memory Architecture Improvements:**
- **Hybrid Data Structures**:
    - **List**: Uses a simple slice for small lists (< 32 elements) for up to 2x faster operations and ~85% less memory usage in common cases, transparently converting to a radix-32 vector trie for larger sizes.
    - **Map**: Small-structure fast paths via tiny array-node updates (≤ 8 keys); core Map remains HAMT. `BatchMapBuilder` and its set variant build the whole trie in one pass.
- **Pointer-Based Array Sharing (planned)**: Reduce allocations in `mapHashArrayNode.clone()` via pointer-backed children with copy-on-write
- **Lazy Copy-on-Write**: Arrays shared via pointers until actual modification
- **Cache-Friendly Design**: Improved memory layout for better CPU cache utilization


## Concurrency with immutable collections

Immutable structures are snapshot-based: every mutation returns a new instance; the original remains unchanged. This makes concurrent reads safe without locks.

- Recommended pattern: a single writer goroutine owns the evolving collection and applies updates received via a channel. It then sends immutable snapshots to readers.
- Readers can safely use received snapshots without copying. Structural sharing ensures those snapshots are cheap to create and pass around.
- If you need a single, shared, evolving reference updated by multiple goroutines, synchronize the reference update (mutex or atomic CAS on a pointer). Without that, simultaneous `Enqueue`/`Dequeue` on the same snapshot may race logically (e.g., multiple consumers reading the same head, or lost enqueues).
- Builders are mutable conveniences and are not safe for concurrent use; keep them confined to one goroutine.

### **Batch Builders**

**Complete High-Performance Builder Suite:**
- **`BatchListBuilder`**: list construction that fills the final trie leaves directly and joins them in `List()`; about 4-5x faster than `ListBuilder` at every size measured, and about 150x faster than a persistent `Append` loop at 100k elements (see [Performance Guidelines](#performance-guidelines))
- **`BatchMapBuilder`**: buffers the entries and builds the whole trie in one pass in `Map()`; 1.4-2.2x faster than `MapBuilder` from 9 entries up and level with it up to 8, with about 45% fewer allocations (see [Performance Guidelines](#performance-guidelines))
- **`BatchSetBuilder`** & **`BatchSortedSetBuilder`**: Efficient bulk set construction
- **`StreamingListBuilder`** & **`StreamingMapBuilder`**: Auto-flush with functional operations (filter, transform)
- **`SortedBatchBuilder`**: Optimized sorted map construction with optional sort maintenance

**Functional Programming Features:**
- Stream processing with automatic memory management
- Filter and transform operations for bulk data processing
- Configurable auto-flush thresholds for memory efficiency

### **Enhanced Testing & Validation**

**Comprehensive Test Coverage:**
- Extensive benchmark suite measuring performance improvements
- Memory profiling and allocation analysis
- Race condition testing (`go test -race`) for thread safety validation
- Edge case and error condition testing for all new builders
- Large-scale performance validation (100-100K elements)


### **Architectural Enhancements**

**Thread Safety & Immutability:**
- Lock-free operations with atomic copying
- Structural sharing maintains thread safety
- Zero-overhead abstractions (Sets inherit Map optimizations)



## List

The `List` type represents a sorted, indexed collection of values and operates
similarly to a Go slice. It supports efficient append, prepend, update, and
slice operations.


### Adding list elements

Elements can be added to the end of the list with the `Append()` method or added
to the beginning of the list with the `Prepend()` method. Unlike Go slices,
prepending is as efficient as appending.

```go
// Create a list with 3 elements.
l := immutable.NewList[string]()
l = l.Append("foo")
l = l.Append("bar")
l = l.Prepend("baz")

fmt.Println(l.Len())  // 3
fmt.Println(l.Get(0)) // "baz"
fmt.Println(l.Get(1)) // "foo"
fmt.Println(l.Get(2)) // "bar"
```

Note that each change to the list results in a new list being created. These
lists are all snapshots at that point in time and cannot be changed so they
are safe to share between multiple goroutines.

### Updating list elements

You can also overwrite existing elements by using the `Set()` method. In the
following example, we'll update the third element in our list and return the
new list to a new variable. You can see that our old `l` variable retains a
snapshot of the original value.

```go
l := immutable.NewList[string]()
l = l.Append("foo")
l = l.Append("bar")
newList := l.Set(2, "baz")

fmt.Println(l.Get(1))       // "bar"
fmt.Println(newList.Get(1)) // "baz"
```

### Deriving sublists

You can create a sublist by using the `Slice()` method. This method works with
the same rules as subslicing a Go slice:

```go
l = l.Slice(0, 2)

fmt.Println(l.Len())  // 2
fmt.Println(l.Get(0)) // "baz"
fmt.Println(l.Get(1)) // "foo"
```

Please note that since `List` follows the same rules as slices, it will panic if
you try to `Get()`, `Set()`, or `Slice()` with indexes that are outside of
the range of the `List`.



### Searching list elements

You can check if a value exists in the list using `Contains()`. For comparable
types, equality uses `==`. For non-comparable types (e.g., `[]byte`), it falls
back to `reflect.DeepEqual`.

```go
// Basic usage
l := immutable.NewList[int]()
for i := 0; i < 5; i++ { l = l.Append(i) }

fmt.Println(l.Contains(3))  // true
fmt.Println(l.Contains(10)) // false

// Non-comparable example uses DeepEqual under the hood
b := immutable.NewList[[]byte]([]byte("foo"), []byte("bar"))
fmt.Println(b.Contains([]byte("foo"))) // true
```

For full control and best performance on custom types, use `ContainsFunc()` to
provide an equality function:

```go
type node struct{ v int }
l := immutable.NewList[*node](&node{v: 1}, &node{v: 2})

eq := func(a, b *node) bool { return a != nil && b != nil && a.v == b.v }
fmt.Println(l.ContainsFunc(&node{v: 2}, eq)) // true
fmt.Println(l.ContainsFunc(&node{v: 3}, eq)) // false
```

### Iterating lists

Iterators provide a clean, simple way to iterate over the elements of the list
in order. This is more efficient than simply calling `Get()` for each index.

Below is an example of iterating over all elements of our list from above:

```go
itr := l.Iterator()
for !itr.Done() {
	index, value, _ := itr.Next()
	fmt.Printf("Index %d equals %v\n", index, value)
}

// Index 0 equals baz
// Index 1 equals foo
```

By default iterators start from index zero, however, the `Seek()` method can be
used to jump to a given index.


### Efficiently building lists

If you are building large lists, it is significantly more efficient to use the
`ListBuilder`. It uses nearly the same API as `List` except that it updates
a list in-place until you are ready to use it. This can improve bulk list
building by 17-28x (1k-100k elements, compared with a loop of `Append` calls).

The [Advanced Batch Builders](#advanced-batch-builders) buffer appends and flush
them in batches. They help only for some input sizes; see their Performance
notes before choosing one over `ListBuilder`.

```go
b := immutable.NewListBuilder[string]()
b.Append("foo")
b.Append("bar")
b.Set(2, "baz")

l := b.List()
fmt.Println(l.Get(0)) // "foo"
fmt.Println(l.Get(1)) // "baz"
```

Builders are invalid after the call to `List()`.


## Queue

An immutable FIFO queue implemented using the classic Okasaki two-list representation backed by cons-lists. Enqueue is O(1). Dequeue and Peek are amortized O(1) when each queue version is used once (the usual `q = q.Enqueue(x)` / `q, v, ok = q.Dequeue()` pattern); the periodic O(n) reversal of the back list is amortized across n operations. The reversal is not cached, so calling `Peek` or `Dequeue` repeatedly on the *same* queue value whose front list is empty costs O(n) every time. Queues built with `NewQueue` or `NewQueueOf` start with a populated front list.

```go
q := immutable.NewQueue[int]()
q = q.Enqueue(1).Enqueue(2).Enqueue(3)

v, ok := q.Peek() // v=1, ok=true

q, v, ok = q.Dequeue() // v=1, ok=true
q, v, ok = q.Dequeue() // v=2, ok=true

itr := q.Iterator()
for !itr.Done() {
    idx, x, _ := itr.Next()
    _ = idx
    _ = x
}

// Builder
b := immutable.NewQueueBuilder[int]()
b.Enqueue(10)
b.EnqueueSlice([]int{20, 30})
finalQ := b.Queue()
```

Thread safety: operations return new queues; existing instances are never mutated, so concurrent reads are safe.

## Map

The `Map` represents an associative array that maps unique keys to values. It
is implemented to act similarly to the built-in Go `map` type. It is implemented
as a [Hash-Array Mapped Trie](https://lampwww.epfl.ch/papers/idealhashtrees.pdf).

Maps require a `Hasher` to hash keys and check for equality. There are built-in
hasher implementations for most primitive types such as `int`, `uint`, and
`string` keys. You may pass in a `nil` hasher to `NewMap()` if you are using
one of these key types.

### Setting map key/value pairs

You can add a key/value pair to the map by using the `Set()` method. It will
add the key if it does not exist or it will overwrite the value for the key if
it does exist.

Values may be fetched for a key using the `Get()` method. This method returns
the value as well as a flag indicating if the key existed. The flag is useful
to check if a `nil` value was set for a key versus a key did not exist.

```go
m := immutable.NewMap[string,int](nil)
m = m.Set("jane", 100)
m = m.Set("susy", 200)
m = m.Set("jane", 300) // overwrite

fmt.Println(m.Len())   // 2

v, ok := m.Get("jane")
fmt.Println(v, ok)     // 300 true

v, ok = m.Get("susy")
fmt.Println(v, ok)     // 200, true

v, ok = m.Get("john")
fmt.Println(v, ok)     // nil, false
```


### Removing map keys

Keys may be removed from the map by using the `Delete()` method. If the key does
not exist then the original map is returned instead of a new one.

```go
m := immutable.NewMap[string,int](nil)
m = m.Set("jane", 100)
m = m.Delete("jane")

fmt.Println(m.Len())   // 0

v, ok := m.Get("jane")
fmt.Println(v, ok)     // nil false
```


### Iterating maps

Maps are unsorted, however, iterators can be used to loop over all key/value
pairs in the collection. Unlike Go maps, iterators are deterministic when
iterating over key/value pairs.

```go
m := immutable.NewMap[string,int](nil)
m = m.Set("jane", 100)
m = m.Set("susy", 200)

itr := m.Iterator()
for !itr.Done() {
	k, v := itr.Next()
	fmt.Println(k, v)
}

// susy 200
// jane 100
```

Note that you should not rely on two maps with the same key/value pairs to
iterate in the same order. Ordering can be insertion order dependent when two
keys generate the same hash.


### Efficiently building maps

If you are executing multiple mutations on a map, it can be much more efficient
to use the `MapBuilder`. It uses nearly the same API as `Map` except that it
updates a map in-place until you are ready to use it.

For enhanced performance with bulk operations, see the 
[Advanced Batch Builders](#advanced-batch-builders) section which provides 
additional optimizations and functional programming capabilities.

```go
b := immutable.NewMapBuilder[string,int](nil)
b.Set("foo", 100)
b.Set("bar", 200)
b.Set("foo", 300)

m := b.Map()
fmt.Println(m.Get("foo")) // "300"
fmt.Println(m.Get("bar")) // "200"
```

Builders are invalid after the call to `Map()`.


### Implementing a custom Hasher

If you need to use a key type besides `int`, `uint`, or `string` then you'll
need to create a custom `Hasher` implementation and pass it to `NewMap()` on
creation.

Hashers are fairly simple. They only need to generate hashes for a given key
and check equality given two keys.

**Security Note:** A poorly implemented `Hasher` can result in frequent hash
collisions, which will degrade the `Map`'s performance from O(log n) to O(n),
making it vulnerable to algorithmic complexity attacks (a form of Denial of Service).
Ensure your `Hash` function provides a good distribution.

```go
type Hasher[K any] interface {
	Hash(key K) uint32
	Equal(a, b K) bool
}
```

Please see the internal `intHasher`, `uintHasher`, `stringHasher`, and
`byteSliceHasher` for examples.


## Sorted Map

The `SortedMap` represents an associative array that maps unique keys to values.
Unlike the `Map`, however, keys can be iterated over in-order. It is implemented
as a B+tree.

Sorted maps require a `Comparer` to sort keys and check for equality. There are
built-in comparer implementations for `int`, `uint`, and `string` keys. You may
pass a `nil` comparer to `NewSortedMap()` if you are using one of these key
types.

The API is identical to the `Map` implementation. The sorted map also has a
companion `SortedMapBuilder` for more efficiently building maps.


### Implementing a custom Comparer

If you need to use a key type besides `int`, `uint`, or `string` or derived types, then you'll
need to create a custom `Comparer` implementation and pass it to
`NewSortedMap()` on creation.

**Security Note:** A slow `Comparer` implementation can severely degrade the
performance of all `SortedMap` operations, making it vulnerable to Denial of Service
attacks. Ensure your `Compare` function is efficient.

Comparers on have one method—`Compare()`. It works the same as the
`strings.Compare()` function. It returns `-1` if `a` is less than `b`, returns
`1` if a is greater than `b`, and returns `0` if `a` is equal to `b`.

```go
type Comparer[K any] interface {
	Compare(a, b K) int
}
```

Please see the internal `defaultComparer` for an example, bearing in mind that it works for several types.

## Set

The `Set` represents a collection of unique values, and it is implemented as a
wrapper around a `Map[T, struct{}]`.

Like Maps, Sets require a `Hasher` to hash keys and check for equality. There are built-in
hasher implementations for most primitive types such as `int`, `uint`, and
`string` keys. You may pass in a `nil` hasher to `NewSet()` if you are using
one of these key types.


## Sorted Set

The `SortedSet` represents a sorted collection of unique values.
Unlike the `Set`, however, keys can be iterated over in-order. It is implemented
as a B+tree.

Sorted sets require a `Comparer` to sort values and check for equality. There are
built-in comparer implementations for `int`, `uint`, and `string` keys. You may
pass a `nil` comparer to `NewSortedSet()` if you are using one of these key
types.

The API is identical to the `Set` implementation.


## Advanced Batch Builders

This library provides batch builders for buffered and streaming construction. They
collect the values first and build the whole collection once, when you ask for
it, instead of changing the collection on every call. Compared with a loop of
persistent operations they are much faster, and `BatchListBuilder` and
`BatchMapBuilder` are also faster than the plain `ListBuilder` / `MapBuilder`.
The measured numbers are in each section's Performance note and in
[Performance Guidelines](#performance-guidelines).

### Batch List Builder

The `BatchListBuilder` writes values straight into the 32-wide leaf nodes of the
final list and joins the leaves in `List()`. `Append` is an array store, no value
is copied twice, and no path is copied through the trie, so it is faster than
`ListBuilder` at every size measured:

```go
// The argument is a hint for how many values you expect (0 selects a default).
// It is never a limit; the builder grows as needed.
builder := immutable.NewBatchListBuilder[int](10000)

// Add many elements efficiently
for i := 0; i < 10000; i++ {
    builder.Append(i)
}

// Or add slices efficiently
values := []int{1, 2, 3, 4, 5}
builder.AppendSlice(values)

list := builder.List()
```

**Performance** (Apple M1 Pro, sequential ints, `-count=10`, see
`memory-bank/reports/bench-baseline-2026-11-17.txt`):

| Elements | `Append` loop | `ListBuilder` | `BatchListBuilder` | `NewList(values...)` |
|---|---|---|---|---|
| 1,000 | 284 µs | 17 µs | 4.3 µs | 2.4 µs |
| 10,000 | 4.3 ms | 200 µs | 44 µs | 22 µs |
| 100,000 | 75 ms | 2.7 ms | 0.5 ms | 0.27 ms |

At 100,000 elements `BatchListBuilder` allocates 1.07 MiB per build against
0.94 MiB for `ListBuilder`. The numbers for `BatchListBuilder` do not depend on
the size argument (batch sizes 16, 128 and 1024 were all measured within noise
of each other), and `NewList(values...)` is faster still when you already have a
slice. Earlier releases flushed in batches and copied the whole list on every
flush, so a small batch size was very slow (265 ms at 100,000 elements with
batch size 16); that no longer applies. Measured on a machine under heavy load, so treat differences of about 20% or
less as noise; the effects above are 4x or larger.

### Batch Map Builder

The `BatchMapBuilder` buffers the entries and builds the whole map in `Map()`. It
sorts the entries by the hash segments the trie uses, one level at a time, so each
node is allocated once at its final size and no entry walks the trie from the
root. The result is the map that calling `Set` for each entry in order would
give: the last value for a key wins, and keys are compared with the `Hasher`.

```go
// The argument is a hint for how many entries you expect (0 selects a default).
// It is never a limit; the builder grows as needed.
builder := immutable.NewBatchMapBuilder[string, int](nil, 10000)

// Add many entries efficiently
for i := 0; i < 10000; i++ {
    builder.Set(fmt.Sprintf("key-%d", i), i)
}

// Or add from existing maps
entries := map[string]int{"a": 1, "b": 2, "c": 3}
builder.SetMap(entries)

m := builder.Map()
```

If `hasher` is nil, the default hasher is chosen from the first key you `Set`.
After `Map()` every method except `Reset()` panics, as with `MapBuilder`.
`Len()` counts `Set` calls, so it is an upper bound on the length of the map; a
key that was set twice counts twice. `Flush()` is a deprecated no-op.

**Performance** (Apple M1 Pro, `MapBuilder` and `BatchMapBuilder` built from the
same keys, back to back, in the same process; the machine was heavily loaded, so
the ratios are from an interleaved comparison of 200 rounds rather than from
separate benchmark runs; see
`memory-bank/reports/bench-phase2-2-batchmap-ab-2026-11-17.txt`):

| Entries | random int keys | sequential int keys |
|---|---|---|
| 4 | same | same |
| 8 | same | same |
| 9-16 | 1.5x faster | 2.2x faster |
| 100 | 1.5x faster | 1.7-1.9x faster |
| 1,000 | 1.4x faster | 1.7x faster |
| 10,000 | 1.4x faster | 1.4x faster |
| 100,000 | 2.1x faster | 1.4x faster |

Up to 8 entries a map is a single array node and the two builders do the same
work, so there is no difference. Allocations drop by about 45% (100,000 random
keys: 294,000 to 163,000). Bytes allocated go up by 10-50% (100,000 random keys: 8.1 to 11.4 MB),
because the buffer and the sort scratch space exist next to the finished map for
the duration of the build. The numbers do not depend on the size argument.
Before this release a single large batch produced one oversized array node, so
`Get` on such a map was a linear scan; the map is now a normal trie at any size.

One difference from building with `Set`: entries whose keys share a full 32-bit
hash are kept in a collision node in first-set order, which can differ from the
order a `Set`-built map has after its root array node was expanded. Lookups and
iteration results do not depend on it.

### Streaming Builders

Streaming builders provide auto-flush capabilities and functional operations:

```go
// Streaming list builder with auto-flush at 1000 elements
builder := immutable.NewStreamingListBuilder[int](32, 1000)

// Functional operations
data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

// Filter even numbers
builder.Filter(data, func(x int) bool { return x%2 == 0 })

// Transform by doubling
builder.Transform(data, func(x int) int { return x * 2 })

list := builder.List() // Contains processed elements
```

```go
// Streaming map builder with auto-flush and bulk operations
builder := immutable.NewStreamingMapBuilder[int, string](nil, 32, 500)

// Add individual entries (auto-flushes at 500 elements)
for i := 0; i < 1000; i++ {
    builder.Set(i, fmt.Sprintf("value-%d", i))
}

// Bulk operations with auto-flush
builder.SetMany(map[int]string{10: "ten", 20: "twenty", 30: "thirty"})

m := builder.Map()
```

### Batch Set Builders

Set builders provide efficient bulk set construction:

```go
// Batch set builder
builder := immutable.NewBatchSetBuilder[string](nil, 64)

values := []string{"apple", "banana", "cherry", "apple"} // "apple" duplicate
builder.AddSlice(values)

set := builder.Set() // Contains 3 unique values

// Sorted set builder with sort maintenance
sortedBuilder := immutable.NewBatchSortedSetBuilder[int](nil, 32, true)
numbers := []int{5, 2, 8, 1, 9, 3}
sortedBuilder.AddSlice(numbers)

sortedSet := sortedBuilder.SortedSet() // Automatically sorted: [1, 2, 3, 5, 8, 9]
```

### Sorted Batch Builder

For sorted maps, use `SortedBatchBuilder` with optional sort maintenance:

```go
// Maintain sort order in buffer for optimal insertion
builder := immutable.NewSortedBatchBuilder[int, string](nil, 32, true)

// Add in random order - automatically maintained in sorted buffer
builder.Set(3, "three")
builder.Set(1, "one")
builder.Set(2, "two")

sm := builder.SortedMap() // Efficiently constructed sorted map
```

### Performance Guidelines

**Choosing a builder (measured, see the notes above):**
- If you do not need buffering or streaming, `ListBuilder` and `MapBuilder` are the safe default: they are never far from the best option.
- `BatchListBuilder` is faster than `ListBuilder` at every size measured (1k, 10k and 100k elements), by about 4-5x. Use it whenever you only append. If you already hold the values in a slice, `NewList(values...)` is faster again. Use `ListBuilder` when you need to `Set`, `Prepend`, `Slice` or `Get` while building.
- `BatchMapBuilder` is 1.4-2.2x faster than `MapBuilder` from 9 entries up, level with it up to 8, and allocates about 45% fewer objects but 10-50% more bytes (the buffered entries and the sort scratch space). Use it whenever you only add entries; use `MapBuilder` when you need `Get` or `Delete` while building.
- `BatchSetBuilder` builds on `BatchMapBuilder` and behaves the same.
- Streaming builders add auto-flush and functional operations (`Filter`, `Transform`); they are a convenience, not a speed-up.

**Batch size:**
- `BatchListBuilder`: the size argument is only a sizing hint and does not change the speed.
- `BatchMapBuilder`: the size argument is only a sizing hint and does not change the speed.

These are measurements of the current implementation, not guarantees. The
numbers will be refreshed as the batch builders are reworked.


## Contributing

The goal of `immutable` is to provide stable, reasonably performant, immutable
collections library for Go that has a simple, idiomatic API. As such, additional
features and minor performance improvements will generally not be accepted. If
you have a suggestion for a clearer API or substantial performance improvement,
_please_ open an issue first to discuss. All pull requests without a related
issue will be closed immediately.

Please submit issues relating to bugs & documentation improvements.


### What's New (2025-09)

- zero dependencies
- Small-structure fast paths:
  - List: `NewList(values...)` and `BatchListBuilder` build the trie bottom-up in one pass (the 2025-09 batch flush that extended a slice in place was replaced because it copied the whole list on every flush)
  - Map: `BatchMapBuilder` builds the whole trie in one pass in `Map()` (last-write-wins, `Hasher` equality); tiny array-node updates applied in-place when under threshold
- New builder APIs:
  - `(*BatchListBuilder).Reset()` and `(*BatchMapBuilder).Reset()` for builder reuse without reallocations
- Concurrency:
  - Added concurrent read benchmarks and mixed read/write benchmarks (immutable Map vs `sync.Map`)
  - Added concurrency correctness tests (copy-on-write isolation under concurrent readers)

### Current Performance Snapshot

Reference machine: Apple M1 Pro, go1.27.4, `-count=10 -benchtime=200ms`. Full data:
`memory-bank/reports/bench-baseline-2026-11-17.txt`; rerun with `scripts/bench.sh run`.
The run was taken on a busy machine (noise about ±2-19%), so treat differences
under about 20% as noise.

Building 100,000 elements (ns per element; `rand` = random int keys, `seq` = 0..N-1):

| Operation | ns/elem | B/elem (rand) | allocs/elem (rand) |
|---|---|---|---|
| Go map, `m[k] = v` | 38 | 47 | 0.005 |
| `MapBuilder.Set` (seq / rand) | 68 / 129 | 81 | 2.9 |
| `Map.Set` loop (seq / rand) | 603 / 849 | 1710 | 8.0 |
| `ListBuilder.Append` | 27 | 9.7 | 0.03 |
| `List.Append` loop | 754 | 2440 | 5.7 |
| `NewList(values...)` | 25 | 9.6 | 0.03 |

Single operations on a 100,000-element collection:

| Operation | Time | Allocations |
|---|---|---|
| `Map.Get` (seq / rand) | 22 ns / 50 ns | 0 |
| `Map.Set` of a new key (rand) | 884 ns | 1.85 KiB, 8 allocs |
| `Map.Delete` of an existing key (rand) | 895 ns | 1.81 KiB, 7 allocs |
| `List.Get` | 8.9 ns | 0 |
| `List.Set` | 714 ns | 2.5 KiB, 6 allocs |
| `SortedMap.Get` (rand) | 203 ns | 0 |

Not covered by the numbers above (measured earlier, not re-run for the baseline):
the concurrency figures below.
- Concurrent reads (ns/op, lower is better):
  - 1G: immutable 3.53 vs `sync.Map` 6.03
  - 4G: immutable 2.31 vs `sync.Map` 3.21
  - 16G: immutable 2.39 vs `sync.Map` 3.24
- Mixed read/write (ns/op):
  - 90/10 (9R/1W): immutable 26.0 vs `sync.Map` 38.4
  - 70/30 (7R/3W): immutable 24.6 vs `sync.Map` 65.0
  - 50/50 (5R/5W): immutable 27.3 vs `sync.Map` 47.4

### New APIs (Builders)

```go
// Reuse list builder across batches without reallocations
lb := immutable.NewBatchListBuilder[int](64)
// ... append ...
lb.Reset() // discards the values and makes the builder valid again

// Reuse map builder and retain hasher
mb := immutable.NewBatchMapBuilder[string,int](nil, 64)
// ... set ...
mb.Reset() // clears state, preserves hasher & buffer capacity
```

### Benchmarking & Profiling

- Reproducible baseline (10 samples per benchmark, compared with `benchstat`):
```bash
scripts/bench.sh run [label]        # writes memory-bank/reports/bench-<label>-<date>.txt
scripts/bench.sh compare OLD NEW    # benchstat OLD NEW
```
The curated set is every benchmark named `BenchmarkBaseline_*`.

- Run all benchmarks with allocations:
```bash
go test -bench=. -benchmem -count=3 ./...
```

- Profile a representative write-heavy benchmark:
```bash
# CPU and memory profiles (example: Map RandomSet, size=10K)
go test -bench=BenchmarkMap_RandomSet/size-10000 -benchmem -run="^$" \
  -cpuprofile=cpu.prof -memprofile=mem.prof -count=1

# Inspect hotspots
go tool pprof -top cpu.prof
go tool pprof -top -sample_index=alloc_space mem.prof
```

Optional: Enable PGO locally
```bash
# Generate a profile and write default.pgo
go test -bench=. -run="^$" -cpuprofile=cpu.prof -count=1
go tool pprof -proto cpu.prof > cpu.pb.gz
go tool pgo -compile=local -o default.pgo cpu.pb.gz

# Use the profile for builds/tests (Go 1.21+)
go test -bench=. -benchmem -count=3 -pgo=auto ./...
```

- Compare immutable vs `sync.Map` concurrent reads:
```bash
go test -bench=BenchmarkConcurrentReads -benchmem -run="^$" -count=1
```

- Mixed workload (reads/writes):
```bash
go test -bench=BenchmarkConcurrentMixed -benchmem -run="^$" -count=1
```

