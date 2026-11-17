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
  - Like `MapBuilder`, every method except `Reset` (including `Len`) now panics
    after `Map()` / `Set()`, and a second `Map()` call panics. Before, a second
    `Map()` returned nil and `BatchSetBuilder.Set()` could return nil. Call
    `Reset()` on a `BatchMapBuilder` to make it valid again.
  - `Len()` counts `Set` / `Add` calls, so it is an upper bound on the size of
    the result (a key that was set twice counts twice). It used to count
    distinct keys already flushed plus buffered entries.
  - When no hasher is given, the default hasher is chosen at the first `Set`,
    so a key type with no default hasher panics there. Before it panicked at the
    first flush, which was the `batchSize`-th `Set` or `Map()`.
- **`StreamingMapBuilder.Stream`, `Filter` and `Transform`** now take an
  `iter.Seq2[K, V]` (for example `maps.All(m)`) instead of the unexported
  `mapEntry` type, which made the methods unusable from outside the package.

### Fixed

- `BatchMapBuilder` compares keys with the `Hasher` on every path. The old code
  used Go equality (a Go map, or `==`) for buffers over 8 entries and when no
  hasher was set yet, which disagrees with a custom `Hasher` whose `Equal` is
  looser than `==`.
- A `BatchMapBuilder` flushed into an empty map with more than 8 buffered
  entries built one oversized array node, so `Get` on the result was a linear
  scan. The result is now a normal trie.
- `Queue` is built on an internal cons-list, so `Enqueue` is O(1) and the
  documented amortized O(1) `Dequeue` holds for linear use. `Peek` and `Dequeue`
  on the same queue value whose front is empty are still O(n) per call; the
  docs now say so.
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
- **`NewList(values...)` builds the trie in one pass** instead of appending one
  value at a time: 2.5 ms to 0.27 ms for 100,000 values (B/op +6%). The trie is
  also one level shallower than one built by `Append`, so `Get` and `Set` on a
  list from `NewList` are 23-39% and 22-42% faster. Values and behavior are
  unchanged.
- Performance claims in the README (19x, 8%, 5.8%, 6-8%) were removed and
  replaced with measured numbers from a reproducible benchmark set
  (`scripts/bench.sh`, benchmarks named `BenchmarkBaseline_*`).
  `BatchMapBuilder` does not consistently outperform `MapBuilder`; see the
  README's Performance Guidelines.
- Benchmarks were corrected so they measure what they claim (deletes of
  present keys, results kept in package-level sinks, inputs generated outside
  the timed region) and Queue benchmarks were added.
