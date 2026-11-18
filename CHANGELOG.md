# Changelog

All notable changes to this project are recorded here. The project follows
semantic versioning, with the usual caveat that versions before 1.0 may include
breaking changes in minor releases.

## Unreleased

Changes since v0.6.0.

### Breaking changes

- **Go 1.26 or later is required.** The `go` directive in `go.mod` moved from
  1.25.0 to 1.26.0. CI runs on Go 1.26 and 1.27.
- **`SetBuilder` and `SortedSetBuilder`**
  - `Set(val)` was renamed to `Add(val)`. Replace `b.Set(x)` with `b.Add(x)`.
  - `SetBuilder` has a new terminal method `Set() Set[T]` that returns the built
    set and invalidates the builder. (`SortedSetBuilder.SortedSet()` is
    unchanged.)
  - All builder methods now use pointer receivers, and the builders are invalid
    after the terminal call. Always create them with `NewSetBuilder` /
    `NewSortedSetBuilder` and do not copy them.
- **`BatchListBuilder`** was rewritten (see Changed).
  - `Flush` is now a deprecated no-op. Values are no longer committed in
    batches.
  - The size passed to `NewBatchListBuilder` is only a sizing hint; it is never
    a limit.
  - Like `ListBuilder`, every method (including `Len`) now panics after `List()`,
    and a second `List()` call panics. Before, `Len` returned 0 and a second
    `List()` returned nil. Call `Reset()` to make the builder valid again.
- **`BatchMapBuilder`** and **`BatchSetBuilder`** were rewritten (see Changed).
  - `Flush` is now a deprecated no-op. Entries are no longer committed in
    batches.
  - The size passed to `NewBatchMapBuilder` / `NewBatchSetBuilder` is only a
    sizing hint; it is never a limit.
  - Like `MapBuilder`, `Set` / `Add` and `Len` now panic after `Map()` / `Set()`,
    and so does a second `Map()` call. Before, a second
    `Map()` returned nil and `BatchSetBuilder.Set()` could return nil. Call
    `Reset()` on a `BatchMapBuilder` to make it valid again.
  - `Len()` counts `Set` / `Add` calls, so it is an upper bound on the size of
    the result (a key that was set twice counts twice). It used to count
    distinct keys already flushed plus buffered entries.
  - When no hasher is given, the default hasher is chosen at the first `Set`,
    so a key type with no default hasher panics there. Before it panicked at the
    first flush, which was the `batchSize`-th `Set` or `Map()`.
- **`SortedBatchBuilder`** and **`BatchSortedSetBuilder`** were rewritten (see
  Changed and Fixed).
  - The `maintainSort` argument was removed:
    `NewSortedBatchBuilder(comparer, batchSize, maintainSort)` is now
    `NewSortedBatchBuilder(comparer, batchSize)`, and
    `NewBatchSortedSetBuilder(comparer, batchSize, maintainSort)` is now
    `NewBatchSortedSetBuilder(comparer, batchSize)`. The entries are always
    sorted once, when the map is built.
  - `Flush` is now a deprecated no-op, and the size is only a sizing hint.
  - Like `SortedMapBuilder`, `Set` / `Add` and `Len` panic after `SortedMap()` /
    `SortedSet()`, and so does a second call; call `Reset()` on a
    `SortedBatchBuilder` to make it valid again. Before, a second `SortedMap()`
    returned nil and `SortedSet()` returned nil.
  - `Len()` counts `Set` / `Add` calls, so it is an upper bound on the size of
    the result. It used to count distinct keys flushed plus buffered entries.
  - When no comparer is given, the default comparer is chosen at the first
    `Set`, so a key type with no default comparer panics there.
- **`StreamingListBuilder` and `StreamingMapBuilder` lost auto-flush.** Because
  the batch builders no longer flush, the `autoFlushSize` argument had no effect
  and was removed: `NewStreamingListBuilder(batchSize, autoFlushSize)` is now
  `NewStreamingListBuilder(batchSize)` and
  `NewStreamingMapBuilder(hasher, batchSize, autoFlushSize)` is now
  `NewStreamingMapBuilder(hasher, batchSize)`. `StreamingMapBuilder.SetMany` is
  deprecated in favor of `SetMap`, which it now calls.
- **`StreamingMapBuilder.Stream`, `Filter` and `Transform`** now take an
  `iter.Seq2[K, V]` (for example `maps.All(m)`) instead of the unexported
  `mapEntry` type, which made the methods unusable from outside the package.

### Fixed

- `SortedBatchBuilder` with `maintainSort` set kept the **first** value of a key
  that was set twice (`Set(1, "a"); Set(1, "b")` gave "a"), unlike every other
  path and unlike `SortedMapBuilder`. The last value wins on every path now.
- `SortedBatchBuilder` with `maintainSort` set ordered its buffer by Go's `<` on
  the key instead of by the `Comparer`. When a custom `Comparer` treated two
  different keys as equal (for example a case-insensitive one), which key and
  value won was decided by their natural order, not by which was set last
  (`Set("a", 1); Set("A", 2)` kept `"a"` with 1). The `Comparer` now decides
  order and key equality everywhere, the last `Set` wins key and value, and the
  key type is `any` instead of `cmp.Ordered`, so keys that `<` cannot order work
  with a `Comparer`.
- `QueueBuilder`, `SetBuilder` and `SortedSetBuilder` record "already built" in
  a flag instead of a nil pointer, so a builder that was never initialised is no
  longer mistaken for a used one. A zero-value `SetBuilder` or `SortedSetBuilder`
  panicked with "builder invalid after Set() invocation" although `Set()` was
  never called; it now panics saying that the zero value is not usable and the
  builder must be created with `NewSetBuilder` / `NewSortedSetBuilder`. A
  zero-value `QueueBuilder` panicked the same misleading way; it is now an empty
  builder that works (a nil `*Queue` already means an empty queue in this
  package), and `Queue()` on one that was never used returns an empty queue, not
  nil.
- `BatchMapBuilder` compares keys with the `Hasher` on every path. The old code
  used Go equality (a Go map, or `==`) for buffers over 8 entries and when no
  hasher was set yet, which disagrees with a custom `Hasher` whose `Equal` is
  looser than `==`.
- A `BatchMapBuilder` flushed into an empty map with more than 8 buffered
  entries built one oversized array node, so `Get` on the result was a linear
  scan. The result is now a normal trie.
- `Queue` is built on an internal cons-list, so `Enqueue` is O(1) and the
  documented amortized O(1) `Dequeue` holds for linear use. `Peek`, `Dequeue`
  and `Iterator` on the same queue value whose front was empty used to repeat the
  O(n) reversal on every call; the reversal is now cached (see Changed).
- `List.Append` (and `ListBuilder.Append`) added a trie level 32 times too
  early, so every list grown by appending was one level deeper than its size
  needs (34 values had a depth-2 trie where depth 1 holds 1,024). The size of
  a list is now compared with the number of values a root can hold
  (32^(depth+1)). Lists grown by `Append` are 1.3-1.6x faster to read with `Get`,
  persistent `Set` is 1.3-1.5x faster with one allocation and 22-39% fewer bytes,
  and building 100,000 values with `Append` allocates 24% fewer bytes. Values and
  behavior are unchanged.
- The zero value of `List` (`var l immutable.List[int]`) is an empty list, as
  documented, for every method. `Append` and `Prepend` on it used to panic with a
  nil dereference.
- Corrected documentation: the default hasher and comparer cover integer and
  string types only (there is no byte-slice default), the `MapBuilder.Map` and
  `SortedMapBuilder.Map` docs and panic messages, and the "must set hasher"
  panic text.

### Changed

- **`BatchListBuilder` is faster than `ListBuilder` at every size measured.**
  It fills the final trie leaves directly and joins them in `List()`. Before,
  every flush copied the whole list, so building 100,000 elements with batch
  size 16 took 265 ms and 2.4 GiB; now it takes 0.5 ms and 1.07 MiB
  (`ListBuilder`: 2.7 ms, 0.94 MiB). `BatchListBuilder` no longer produces slice
  nodes larger than 32 elements.
- **`BatchMapBuilder` is 1.4-2.2x faster than `MapBuilder` from 9 entries up**
  and level with it up to 8 (random and sequential int keys, 9 to 100,000
  entries, measured back to back in one process). `Map()` partitions the
  buffered entries by the hash segments the trie uses and allocates every node
  once at its final size. Allocations drop by about 45% (100,000 random keys:
  294,000 to 163,000). Bytes allocated rise by 10-50% (8.1 to 11.4 MB at
  100,000), because the buffer and the sort scratch space exist next to the
  result while it is built. The old builder's time depended on the size
  argument (10,000 entries took 0.9 ms at batch size 128 and 5.0 ms at 1024);
  the new one's does not.
  `BatchSetBuilder` builds on it and gets the same gain. Entries whose keys
  share a full 32-bit hash may be kept in a different order inside their
  collision node than `Set` would give; lookups and iteration are unaffected.
- **`SortedBatchBuilder` is faster than `SortedMapBuilder`**, which the old one
  never was (it was 0.7-1.0x). `SortedMap()` sorts the entries once, not at all
  when they were added in strictly increasing order, and cuts them into leaves
  and branches, so every node is allocated once at its final size. Back to back
  in one process: 1.1-1.4x faster for random keys (100 to 100,000 entries),
  1.9-6.8x faster for increasing keys, level at 8 entries or fewer; allocations
  drop 87-94% (100,000 random keys: 116,900 to 6,474). Bytes allocated rise to
  1.2-2.5x because the buffer exists next to the result while it is built.
  Leaves are balanced (33 entries become 16 + 17) and no node is underfull.
- **`NewList(values...)` builds the trie in one pass** instead of appending one
  value at a time: 2.5 ms to 0.27 ms for 100,000 values (B/op +6%). The trie is
  also one level shallower than one built by `Append`, so `Get` and `Set` on a
  list from `NewList` are 23-39% and 22-42% faster. Values and behavior are
  unchanged.
- **`List` keeps its last values in a tail outside the trie** (ADR-005). `Append`
  copies at most 32 values and the `List` instead of the path from the root to
  the last leaf, and only touches the trie once per 32 appends, when the full
  tail becomes a leaf. Building 100,000 values with an `Append` loop went from
  1,863 to 256 bytes and from 4.7 to 2.1 allocations per `Append` (7.3x fewer
  bytes) and is 5.6x faster (52.5 ms to 9.3 ms); at 10,000 values it is 4.5x
  faster with 5.9x fewer bytes, at 1,000 values 3.0x and 3.9x. `ListBuilder` is
  2.2-2.9x faster (100,000 values: 1.93 ms to 0.67 ms). `Append` onto a list of
  32 values allocates 360 bytes instead of 2,080, and onto 33 values 80 bytes
  instead of 896. `NewList` of 33 values is 3x faster with 30% of the bytes.
  The slice-backed node for lists of up to 32 values (ADR-004) is replaced by
  the tail, so there is one code path for small and large lists. `Get`,
  iteration and `Contains` are unchanged. The costs: the `List` struct is 32
  bytes larger, so a list of up to 32 values takes 8 more bytes (one value:
  72 B in 2 allocations, was 64 B in 3), and persistent `Set` on a large list
  allocates about 30 bytes more (2,048 to 2,079 at 100,000 values) with one
  allocation fewer. `Set` and `Prepend` on the trie still copy the whole path.
  Values and behavior are unchanged.
- **`ListIterator` is 2.4-3x faster and 3.7x smaller.** It remembers the leaf and
  slot it is on, so `Next` and `Prev` read the slot and step, and it only walks
  the branches above to move to the neighboring leaf, once per 32 values.
  Iterating 100,000 values takes 0.29 ms instead of 0.69 ms (about 2.9 ns per
  value instead of 7), and `Iterator()` allocates 240 bytes instead of 896 (its
  path now has one entry per trie level instead of a fixed 32). `Contains` and
  `ContainsFunc` scan a leaf at a time and are 2.3-3.2x faster on lists with a
  trie. Values, order and behavior are unchanged.
- **Small `Map`s are faster to read and to grow.** A map of up to 8 entries now
  stores the hash of each key, so `Get` compares hashes first and calls the
  hasher's `Equal` only on a match: `Get` of a present key is 1.3-1.9x faster
  with `int` keys and 1.5-2.5x faster with `string` keys at 4 to 8 entries. The
  `Set` that takes a map from 8 to 9 entries builds the trie from the stored
  hashes instead of hashing every key again, and builds it in place: building a
  map of 9 entries is 1.37x faster with 18% fewer allocations (61 to 50). The
  same hash-first compare is used inside the trie, where it saves one `Equal`
  call per lookup but is level in time (large-map `Get`, `Set` and `Delete`
  measured at 1,000 to 100,000 entries). The costs: each entry of a map of up to
  8 entries is 8 bytes larger (a map of 4 entries takes 18% more bytes, of 8
  entries 26% more), and the first `Set` into an empty map now hashes its key,
  so a map of one entry takes about 8-10% longer to build (and `Get` on it
  measured about 1 ns slower, within the noise). Values, order and behavior are
  unchanged.
- **The default hash for `string` keys is rapidhash, so the iteration order of
  string-keyed `Map`s and `Set`s changes once, when you upgrade.** The old hash
  (`31*h + c`) gave `"Aa"` and `"BB"` the same value, and so every string made by
  joining them: 1,024 such strings shared one hash and one linearly scanned
  collision node. rapidhash (a port of `go.dw1.io/rapidhash` v0.3.0 in
  `rapidhash.go`, MIT, no new dependency, no `unsafe`) gives them distinct hashes
  and spreads real keys like a random 32-bit hash (about 1.2 full-hash collisions
  per 100,000 keys). The order is still the same on every run and platform, but a
  test or example that prints a string-keyed map in iteration order needs new
  expected output. The hash itself is 1.7x faster for 8-byte keys, 2.9x for 16 bytes,
  6.6x for 64 bytes and 16x for 1 KB, and 36-byte uuid-like keys read 1.4-2.1x
  faster in a `Map` (the larger key sizes were not measured in a `Map`).
  Short, dense keys such as `"key-1"`, `"key-2"` are about 15-25% slower to read at
  1,000 entries, because the old hash gave consecutive keys consecutive hashes
  and a compact trie; they are level to slightly faster at 10,000 and 100,000
  entries when read in random order. `reflect`-based hashing of named string types
  uses the same function. The default hasher is still deterministic and not
  resistant to hash flooding; use a custom seeded `Hasher` for keys an attacker
  controls.
- **`Map.Set` and `Map.Delete` allocate one object less.** The internal node
  methods return whether a key was added or removed instead of writing through a
  pointer that escaped to the heap. Time is unchanged within measurement noise.
- **`NewHasher` picks the hasher for the key type once.** The built-in hasher no
  longer switches on the type of the key on every `Hash` call, and `Equal`
  compares the keys directly instead of boxing both in an interface. `Get` on a
  map with `int` or `string` keys is 1.02-1.30x faster (1.09-1.66x on maps of up
  to 16 entries), a miss 1.00-1.16x faster; `Set` and `Delete` are level, and bytes
  and allocations are unchanged. Hash values and iteration order are unchanged.
  The unexported `defaultHasher` is replaced by `integerHasher` and `stringHasher`.
- **`Queue` caches the reversal of its back list.** The first `Peek`, `Dequeue`
  or `Iterator` on a queue that has only a back list (the shape an `Enqueue`
  loop makes) turns the list around once and keeps the result, so reading the
  same queue value again costs O(1) instead of O(n): at 10,000 elements `Peek`
  went from 142 us to 2.6 ns with no allocation, `Dequeue` from 143 us to 16 ns,
  and a second pass with `Iterator` is 6x faster with 3 allocations instead of
  10,002. The first read is as fast as before. The price is memory: a queue
  that has been read this way holds a second copy of its elements for as long
  as it lives. The copy is stored atomically, so concurrent readers are safe
  and share it or each make an equal one.
- `QueueBuilder` collects the values in a slice and makes the queue once, with
  all of them in the front list: building is 1.3-1.45x faster with half the
  allocations (10,000 elements: 20,002 to 10,013; bytes +4.6% for the slice that
  exists while the builder is open), and a built queue does not need a reversal
  to be dequeued (building and draining 10,000 elements is 1.6x faster).
  `EnqueueSlice` copies the values, as before.
- `NewQueue` and `NewQueueOf` build the list directly instead of copying the
  input first (1.1x faster, one allocation and 25% of the bytes fewer).
  `QueueIterator` walks the front and then the back through one cursor and is
  1.1-1.2x faster with 25% fewer bytes. The length of a queue is now taken from
  its lists, not stored next to them, so the two cannot disagree.
- Performance claims in the README (19x, 8%, 5.8%, 6-8%) were removed and
  replaced with measured numbers from a reproducible benchmark set
  (`scripts/bench.sh`, benchmarks named `BenchmarkBaseline_*`).
  `BatchMapBuilder` does not consistently outperform `MapBuilder`; see the
  README's Performance Guidelines.
- Benchmarks were corrected so they measure what they claim (deletes of
  present keys, results kept in package-level sinks, inputs generated outside
  the timed region) and Queue benchmarks were added.
