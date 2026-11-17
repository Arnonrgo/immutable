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
- **`StreamingMapBuilder.Stream`, `Filter` and `Transform`** now take an
  `iter.Seq2[K, V]` (for example `maps.All(m)`) instead of the unexported
  `mapEntry` type, which made the methods unusable from outside the package.

### Fixed

- `BatchMapBuilder.Flush` now applies last-write-wins consistently, including on
  the large-buffer path.
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
