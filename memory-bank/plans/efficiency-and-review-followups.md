# Plan: Efficiency & Review Follow-ups

Source: `/review` of commit 576067d and `/vet` (efficiency focus), 2026-11-17.
Rule for every phase: measure before and after with the Phase 0 baseline (`benchstat`), and do not claim a speedup in the README unless the numbers back it.

Status legend: [ ] todo, [~] in progress, [x] done

---

## Phase 0: Benchmark ground truth (first, blocks everything else)

Goal: trustworthy, repeatable numbers so each later phase can be judged against a baseline.

- [x] 0.1 Fix `Map_RandomDelete` and `Set_Operations/Delete`. They delete keys that are already gone (about 2.3 ns, 0 allocs). Re-seed per iteration, using `b.StopTimer`/`StartTimer` or a pre-built persistent map plus a key sequence that is always present.
- [x] 0.2 Fix `immutable_test.go:546,553` (SA4010). The `append` results are never used, so those benchmarks measure nothing. Use the result, or a package-level sink.
- [x] 0.3 Add a package-level sink to all benchmarks that discard results, so the compiler can't elide the work.
- [x] 0.4 Add Queue benchmarks:
  - Enqueue
  - Dequeue (linear use)
  - Peek on a fresh queue, and Peek repeated on the same queue value (the persistent-reuse case)
  - Iterator
  - QueueBuilder
  - NewQueueOf
- [x] 0.5 Add a baseline set that matches the claims:
  - List Append at N=1k, 10k and 100k (persistent loop)
  - ListBuilder
  - BatchListBuilder at batch sizes 16, 128 and 1024
  - NewList
  - MapBuilder vs BatchMapBuilder vs a direct `Set` loop
  - Sequential vs random int keys (the hash-quality case)
  - String keys
- [x] 0.6 Replace single-shot `-benchtime=200ms -count=1` runs with `-count=10` and `benchstat`. Add a `make bench` / `scripts/bench.sh` target that writes `memory-bank/reports/bench-baseline-<date>.txt`.
- [x] 0.7 Commit the baseline file (82b7e7c, local branch perf/benchmark-baseline-and-doc-truth, not pushed). Every later phase compares against it, and its PR records the benchstat diff.
- [x] 0.8 Clean up the remaining test lint: unused `veryVerbose` and `mockComparer` (U1000), SA4006 at `immutable_test.go:1954`, ST1023 in `set_benchmarks_test.go:13`.

Exit: `go vet` and `staticcheck` are clean on tests, and the baseline file is committed. (Status 2026-11-17: all done; 0.7 committed locally in 82b7e7c, not pushed.)

### Baseline results (2026-11-17, `memory-bank/reports/bench-baseline-2026-11-17.txt`)

Apple M1 Pro, go1.27.4, commit 576067d plus the uncommitted Phase 0 changes, `-count=10 -benchtime=200ms`. The machine was busy (load average 44-85 on 10 cores), so per-benchmark noise is ±2-19% (worst ±24%). Differences under about 20% are noise; every finding below is 5x or larger.

| Finding | Numbers (N=100k unless noted) |
|---|---|
| Persistent `List.Append` is 28x slower than `ListBuilder` and allocates 250x more | 754 ns/elem, 232.6 MiB, 569k allocs vs 26.9 ns/elem, 942 KiB, 3.3k allocs |
| `BatchListBuilder` beats `ListBuilder` only when the whole input fits in a few large batches; otherwise it loses, and small batches are worse than a persistent `Append` loop | batch 1024: 4.7 µs vs 16.7 µs (N=1k) and 80 µs vs 201 µs (N=10k) win, but 4.7 ms vs 2.7 ms (N=100k) loses. At N=100k batch 128: 34 ms, batch 16: 265 ms (2.35 GiB), vs 75 ms for the persistent `Append` loop. Cost grows superlinearly with N/batch (flush is O(n)). |
| `BatchMapBuilder` matches `MapBuilder` at batch 16-128 (same 294k allocs at N=100k) and wins only when the whole input is one batch; large batches lose | N=1k batch 1024: 30 µs vs 68 µs (wins, 2.3x); N=10k batch 1024: 4.99 ms vs 0.88 ms (5.7x slower); N=100k batch 16/128: 13.2/12.5 ms vs 12.9 ms (equal) |
| Persistent `Map.Set` loop is 6-7x slower than `MapBuilder`, and 22x slower than a Go map | 849 ns/elem, 163 MiB, 798k allocs vs 129 ns/elem, 7.7 MiB vs 38 ns/elem, 4.5 MiB (rand keys) |
| Sequential-key lookups are 2.3x faster than random-key lookups (identity hash plus cache locality), so key distribution matters | Map.Get at N=100k: 21.6 ns (seq) vs 49.9 ns (rand); MapBuilder at N=100k: 68 vs 129 ns/elem |
| Single persistent ops are fine in time but allocate a lot of small objects | `Map.Set` one key: 884 ns, 1.85 KiB, 8 allocs; `List.Set`: 714 ns, 2.5 KiB, 6 allocs |
| Queue `Peek`/`Dequeue` on a back-only queue is O(n) on reuse (the README claim does not hold for persistent reuse) | N=10k: 145.7 µs, 234 KiB, 10k allocs vs 2.3 ns for a front-populated queue |
| `QueueBuilder` is no faster than a persistent `Enqueue` loop | N=10k: 284 µs vs 271 µs |
| String keys are in the same range as random int keys; the hash cost is not a clear outlier in this run, so 5.3 needs its own hash benchmark | MapString.Get 24.6 ns (N=1k) vs 12.7 ns (seq int) / 27.4 ns (rand int); MapString.Set 765 ns/elem vs 603 (seq) / 849 (rand) |

These numbers set the Phase 2 target (batch builders must beat the plain builders at all N and batch sizes; today they win only when the input fits in one or a few large batches), the Phase 3 target (Peek O(1) on reuse), and the Phase 4 target (List Append B/op at N=100k from 232 MiB to under 23 MiB).

---

## Phase 1: Quick wins and doc truth (low risk)

- [x] 1.1 Go version (decided): keep `go 1.26.0` in `go.mod` (it was 1.25.0 before 576067d). Build and benchmark with the installed toolchain, go1.27.4.
  - Status 2026-11-17: `CHANGELOG.md` created (it did not exist) with the breaking changes since v0.6.0; README states Go 1.26+; `.github/workflows/test.yml` now runs a 1.26.x and 1.27.x matrix, adds `go vet` and a `-race -short` run, and pins `actions/checkout` (v4.4.0) and `actions/setup-go` (v5.6.0) to commit SHAs (the old `@v2` tags were mutable and very old). Verified locally on go1.27.4 only (build, vet, staticcheck, full tests, `-race -short`). **Go 1.26.x is not installed here, so the 1.26 leg is first exercised by CI.** The workflow itself has not been run. `default.pgo` regeneration stays in 6.1.
  - The `go` directive is the minimum version for library consumers, so the 1.25 to 1.26 bump is a visible change. Add a CHANGELOG note, and make sure CI and the README requirements say 1.26+.
  - Run build, vet and `-race` tests on 1.27.4, and confirm they also pass on 1.26.x if that toolchain is available (CI matrix: 1.26 and 1.27).
  - Record the toolchain version in the Phase 0 baseline file header, and regenerate `default.pgo` with 1.27.4 (6.1).
- [x] 1.2 Reword the Queue complexity claim in `README.md` (about line 228) and the `queue.go:3-8` comment to "amortized O(1) when each queue version is used once; worst case O(n) per op on reuse". Remove "at most one cons cell per operation".
- [x] 1.3 Restate the README speedup claims from Phase 0 numbers:
  - Status 2026-11-17: the 19x, 8%, 5.8% and 6-8% claims are removed or replaced with measured tables (README: intro, batch builder sections, Performance Guidelines, Current Performance Snapshot). The README now states that batch builders do not consistently beat `ListBuilder`/`MapBuilder`, with the numbers; Phase 2 will re-measure. Still unmeasured and left as they were: the small-list "up to 2x faster, ~85% less memory" claim (README Hybrid Data Structures, ADR-004) and the ADR-001 "53%" claim. There is no small-list (< 32 elements) baseline benchmark yet, so those move to 6.2 and need one added first. The concurrency figures in the README snapshot are from an earlier session and are labelled as not re-run.
  - BatchListBuilder "19x" (restate against ListBuilder and direct construction)
  - BatchMapBuilder "8% / 5.8%"
  - the "6-8% memory" claim
  - the README benchmark tables (refresh them)
  - the List described as a "HAMT" (it is a radix-32 vector trie; also fix ADR-004 wording)
- [x] 1.4 Fix assert and doc errors:
  - `immutable.go:212` assert text says "SortedMapBuilder.Map()" in `MapBuilder.Map`
  - the `SortedMapBuilder.Map` doc claims the builder stays usable
  - the `NewMap` doc mentions a byte-slice hasher that doesn't exist
  - "must set comparer" in `defaultHasher.Hash` should say hasher
- [x] 1.5 Fix the ADR-003 heading ("ADR-002") and note the numbering gap.
- [x] 1.6 Check README and CHANGELOG for `SetBuilder.Set(val)` examples and note the `Set` to `Add` break. Check README for the `iter.Seq2` builder signatures.
  - Status 2026-11-17: the README has no `SetBuilder`/`SortedSetBuilder` or `StreamingMapBuilder.Stream/Filter/Transform` examples, so nothing needed changing there. The new CHANGELOG records the `Set` to `Add` rename, the new `SetBuilder.Set()` terminal method, the pointer-receiver change and the `iter.Seq2` signatures.
- [ ] 1.7 Commit `memory-bank/plans/code-review-issues.md`.
- [x] 1.8 Hoist `reflect` out of the `List.Contains` per-element loop (`list.go:77-111`) using `reflect.TypeFor[T]().Comparable()` once, and walk leaves instead of a full `Iterator()`.
  - Done 2026-11-17 for the hoist; "walk leaves" is deferred to 4.5 (iterator rewrite). A first attempt (a per-call `func(a, b T) bool`) was measured and rejected: no faster, and +1 alloc/op (16 B), because closures in generic functions capture the type dictionary and escape. The kept version picks an equality mode once and switches per element (`valueEqualityMode`, `valuesEqual`). Result with `BenchmarkBaseline_List_Contains` (10 samples): slice-backed 4.25 to 3.67 ns/elem (-14%), trie-backed 10.7 to 10.0 ns/elem (-5%, near the noise floor), 0 added allocs. The trie case is dominated by the iterator, so 4.5 is where the real win is.
  - Added tests for interface element types (mixed dynamic types), concrete non-comparable structs and concrete comparable structs, on both slice-backed and trie-backed lists.

---

## Phase 2: Builder correctness and efficiency

Decision: the batch builders are kept as efficiency tools, and their API may change. They are only deprecated as a last resort, if after the fix they still can't beat the plain builder in the Phase 0 numbers.

- [x] 2.1 `BatchListBuilder`:
  - Accumulate in a growing `[]T` and bulk-build the trie once in `List()`, with a 32-wide leaf chunker. Target: faster than `ListBuilder` (0.22 ms for 10k) at every batch size.
  - API may change as needed, for example dropping the batch-size knob if bulk-build at `List()` makes it irrelevant.
  - `NewList(values...)` should use the same bulk path.
  - Status 2026-11-17: done, target met. Not built as planned: a first version accumulated in a growing `[]T` and bulk-built in `List()`. It beat `ListBuilder` by about 2.3x in time, but allocated 4.9 MiB per build at N=100k (Go grows large slices by about 1.25x, so the buffer produced about 4x its final size in garbage) against 0.94 MiB for `ListBuilder`. The kept design writes straight into 32-wide `listLeafNode`s (`tail` plus a `leaves` slice) and `List()` only wires branches (`wireListLeaves`, shared with `NewList` via `buildListTrie`). Nothing is copied twice and no oversize slice node is ever produced.
  - API changes (in CHANGELOG): `Flush` is a deprecated no-op; the size argument is only a hint; every method panics after `List()` (as `ListBuilder` does), including `Len`; a second `List()` panics where it used to return nil.
  - Results, same machine under heavy load (load average 85-127 on 10 cores), `-count=10`, vs the committed baseline: `BatchListBuilder` N=100k batch 16: 265 ms, 2.35 GiB -> 0.50 ms, 1.07 MiB; batch 128: 34 ms -> 0.48 ms; batch 1024: 4.7 ms, 37.9 MiB -> 0.52 ms, 1.07 MiB. Against `ListBuilder` in the same run (27 ns/elem, 942 KiB): 4.2x (N=1k), 4.5x (N=10k) and 5.4x (N=100k) faster, at +13% B/op. Batch size makes no measurable difference. `NewList` N=100k: 2.5 ms -> 0.27 ms (B/op +6%: the intermediate leaf slice). allocs/op for the batch builder rose at batch 128 and 1024 (203 -> 3238 at N=100k) because the old version produced one huge slice node (few allocations, quadratic copy) and the new one a real trie.
  - Side effect to remember when comparing later phases: `NewList` now builds a minimum-depth trie, which is one level shallower than an append-built one, so `Baseline_List_Get` and `Baseline_List_Set` (which build their input with `NewList`) moved: Get -23% to -39%, Set -22% to -42% time, -22% to -39% B/op and one fewer allocation. That is not an improvement of Get or Set themselves. Compare Phase 4 against `memory-bank/reports/bench-phase2-1-listops-2026-11-17.txt`, not the original baseline, for those two. `Baseline_List_Append` (an append loop) is unaffected.
  - Reports: `memory-bank/reports/bench-phase2-1-batchlist-v2-2026-11-17.txt` (final), `...-batchlist-2026-11-17.txt` (first, rejected design), `...-listops-2026-11-17.txt`.
  - Tests added: `list_bulk_test.go` (shape and mutation checks around every node-capacity boundary: contents, minimum depth, origin 0, occupancy masks, and Append/Prepend/Set/Slice/Contains on bulk-built lists) and builder lifecycle, aliasing and size tests in `enhanced_builders_test.go`. Verified by mutation: 8 deliberate breakages (wrong occupancy mask, lost last child, wrong branch depth, size off by one, AppendSlice not advancing, Reset not clearing, missing occupied on seal and on the tail) each fail at least one test. One mutation (wrong occupancy mask) first survived because `Get` and the iterator cannot see it, so the structural `occupiedCount` check was added.
- [x] 2.2 `BatchMapBuilder.Flush`:
  - Replace the 130-line dedup logic with a simpler design that is actually faster than `MapBuilder` in the benchmarks (not just within noise). Options: sequential `set(..., true)`, or sort and bulk-load by hash. Keep `TestBatchMapBuilder_LastWriteWins`.
  - Use the user `Hasher` for equality, not `any(a) == any(b)`.
  - API may change as needed.
  - Status 2026-11-17: done, target met. Chosen design: bulk-load by hash. `Set` only appends to an entry buffer (doubled by hand, because `append` leaves about 4x garbage for large slices, the same lesson as 2.1). `Map()` hashes the entries into 8-byte `{hash, idx}` items and partitions them by the trie's 5-bit hash segments, one level at a time, with a stable counting sort (insertion sort for 12 items or fewer), so each node is allocated once at its final size (`map_bulk.go`, about 150 lines in place of the 130-line `Flush`). Groups with an equal full hash are deduplicated with `Hasher.Equal` (last value wins, first-set key order). Results have the shape `Set` gives: bitmap node up to 25 children, hash array node above, array root for 8 distinct keys or fewer. `Flush` is a deprecated no-op; the size argument is a hint only; all methods but `Reset` panic after `Map()`.
  - Also fixed on the way: the old empty-map fast path installed one array node of any size (N=1k at batch 1024 did 20 allocs and had an O(n) `Get`), and used Go equality for buffers over 8 entries.
  - Results (interleaved A/B, because the machine was at load average about 160-180 on 10 cores and separate benchmark runs were not comparable: `MapBuilder` N=100k read 12-70 ms across runs): `BatchMapBuilder` vs `MapBuilder` back to back, 200 rounds, min ratio: random keys 1.00x (N=4), 1.31x (8), 1.53x (9), 1.56x (16), 1.49x (100), 1.43x (1k), 1.43x (10k), 2.13x (100k); sequential keys 1.40x (4), 1.11x (8), 2.18x (9), 2.15x (16), 1.85x (100), 1.65x (1k), 1.37x (10k), 1.45x (100k). At 8 entries or fewer the p25 ratios are about 1.0x, a tie. The goal of "faster, not just within noise" is met from 9 entries up. allocs/op at N=100k: 294,360 -> 163,425 (-44%), N=10k: 27,540 -> 14,687, N=1k: 2,856 -> 1,640. B/op rose 10-50% (N=100k: 8.07 MB -> 11.4 MB; the buffer and the sort scratch are live next to the result). Against the old `BatchMapBuilder` (benchstat, count=10, same load caveat): N=100k batch 16/128/1024 -38%/-45%/-61% time, N=10k batch 1024 -83%. The one regression, N=1k batch 1024 (30 -> 53 µs), is the old oversized array node.
  - Reports: `memory-bank/reports/bench-phase2-2-batchmap-2026-11-17.txt` (benchstat format, count=10), `memory-bank/reports/bench-phase2-2-batchmap-ab-2026-11-17.txt` (interleaved A/B).
  - Tests added: `map_bulk_test.go` (trie invariants checked on every node: array only at the root with 1-8 entries, hash and path prefix of value nodes, collision nodes with 2 or more entries, bitmap popcount and at most 25 children, no single-child bitmap node over a leaf, hash array count of at least 26, max shift; a structural comparison against an incrementally built map; collision groups across hash functions with several moduli and shifts; last-write-wins; custom `Hasher.Equal`; persistent operations on bulk-built maps; the segment sort itself; builder lifecycle). `TestBatchMapBuilder_LastWriteWins` kept (subtests renamed, since `Flush` is a no-op). 22 deliberate breakages were each caught by at least one test (including a Go-equality comparison on `any`, which first failed to compile and was redone).
  - Removed: the `UpdateWithinThreshold` sub-benchmark of `BenchmarkSmallMap_BatchBuilder`, which poked at an internal field that no longer exists.
  - API changes are in CHANGELOG "Breaking changes". Dead logic left for 2.4: `StreamingMapBuilder.Stream` / `SetMany` still call `Flush` at `autoFlushSize`.
- [x] 2.3 `SortedBatchBuilder`:
  - Honor the supplied `Comparer` (it uses `defaultCompare` today).
  - Replace per-Set sorted insertion with a single `slices.SortFunc` at flush, or delete the sorted option.
  - Status 2026-11-17: done, target met. Chosen design: sort once at build, delete the sorted option. `Set` appends `{key, value, seq}` to a buffer (doubled by hand, `appendDoubling`, shared with `BatchMapBuilder`). `SortedMap()` skips the sort when the keys are already strictly increasing, otherwise sorts with `slices.SortFunc` ordered by (Comparer, seq), keeps the last entry of each equal run (key and value, as repeated `Set` does), then cuts the entries into balanced leaves (33 entries become 16 + 17) and branches of at most 32, bottom-up (`sorted_bulk.go`, about 120 lines). The `Comparer` decides order and equality everywhere; the key constraint is `any` instead of `cmp.Ordered`; `maintainSort` is gone; `Flush` is a deprecated no-op; the size argument is a hint only; `Set`/`Len`/a second `SortedMap()` panic until `Reset`. `BatchSortedSetBuilder` follows and no longer returns nil.
  - Bugs found on the way (both in the old code, both fixed and covered by tests): `maintainSort=true` was first-write-wins (`Set(1,"a"); Set(1,"b")` gave "a"), and with a case-insensitive `Comparer` the winner among keys it calls equal was decided by their natural order (`Set("a",1); Set("A",2)` kept `"a"` with 1). Both probed against HEAD before changing anything.
  - A first version used `slices.SortStableFunc` and was rejected after measurement: slower than `SortedMapBuilder` for random keys (N=100k: 25.5 ms vs 20.3 ms). Switching to the unstable sort with a `seq` tiebreak fixed it at the cost of 8 bytes per buffered entry.
  - Results (interleaved A/B in one process against the plain `SortedMapBuilder`, old and new binaries alternated, because the machine was at load average about 200): random keys 1.00x (N=8), 1.13x (100), 1.15x (1k), 1.20x (10k), 1.39x (100k); increasing keys 1.18x (8), 1.90x (100), 4.88x (1k), 6.34x (10k), 6.82x (100k). allocs per build at N=100k: 116,899 -> 6,474 (-94%); bytes 1.16-2.45x (N=100k random: 5.7 -> 7.9 MB). The old `SortedBatchBuilder` was never faster than `SortedMapBuilder` (0.68-1.04x).
  - Report: `memory-bank/reports/bench-phase2-3-sortedbatch-ab-2026-11-17.txt` (includes the raw rows of the rejected stable-sort version). Benchmarks: `BenchmarkBaseline_SortedMap_BatchBuilder`; `BenchmarkSortedBatchBuilder` now measures the build instead of `fmt.Sprintf`.
  - Tests added: `sorted_bulk_test.go` (a B+tree invariant checker: node sizes, strictly increasing keys, branch key = child minimum, equal leaf depth, size, and for bulk builds the fewest leaves and no single-child branch; builds compared with a model and with `Set` one key at a time over sizes on every node-capacity boundary up to 4 levels and eight input orders including duplicates and all-equal; pinned tree shapes; reverse, case-insensitive and struct-key comparers; last-write-wins across `Flush`; persistent `Set`/`Delete` on bulk-built maps with the starting map checked afterwards; Seek and backward iteration; builder lifecycle). Verified by mutation: 23 deliberate breakages (no compaction, first-wins compaction, key-only last-wins, `>=` to `>` in the sorted check, no seq tiebreak, seq always zero or descending, an extra leaf, wrong branch key, wrong size, wrong chunk bounds, comparer never resolved, Reset not clearing or not re-arming, duplicate `SortedMap()` allowed, and others) were each caught. One (Reset keeping old entries) first survived because Reset after `SortedMap()` has nothing left to clear, and `ResetDiscardsPendingEntries` was added.
- [x] 2.4 `StreamingMapBuilder`: make `Stream`, `Filter` and `Transform` consistent about auto-flush, and document the choice.
  - Status 2026-11-17: done. The batch builders no longer flush, so auto-flush had no meaning: `autoFlushSize` was removed from `NewStreamingListBuilder` and `NewStreamingMapBuilder`, which now wrap `BatchListBuilder` / `BatchMapBuilder` and add no state. `StreamingListBuilder.Stream` appends until the channel is closed. `StreamingMapBuilder.SetMany` is a deprecated alias of `SetMap`. README and CHANGELOG say so (breaking-change entry); tests and benchmarks use the new constructors, with `Stream` and empty-channel tests added.
- [x] 2.5 Add an explicit `done` flag to `QueueBuilder`, `SetBuilder` and `SortedSetBuilder`, so the invalid state is not encoded as nil. This also fixes the misleading assert on a zero-value builder.
  - Done. Each builder now has `done bool`, set by the terminal call (which also drops the reference to the built value). The three states are: zero value (never initialised), live, done.
  - `SetBuilder` / `SortedSetBuilder` need a constructed `Set` (and a hasher or comparer), so their zero value stays unusable, but it now panics with its own message ("zero value is not usable, create the builder with NewSetBuilder") instead of claiming `Set()` was called. Both go through one `live()` helper that checks done, then the zero value. Choosing "panic with a clear message" over "make the zero value work" is deliberate: it can be relaxed later without breaking anyone, the reverse cannot.
  - `QueueBuilder` needs no constructor state, because a nil `*Queue` is already the empty queue (`Enqueue`, `Len`, `Peek` handle nil), so its zero value is now a working empty builder, and `Queue()` on a never-used one returns `NewQueue()` rather than nil.
  - Tests: `builder_lifecycle_test.go` checks the panic message of every method after the terminal call, on an empty builder, and on a zero value (it checks the message because the message was the bug). 22/22 mutations killed (dropping a done check, leaving done unset, bypassing `live()`, wrong or swapped messages, nil from an unused zero `QueueBuilder`, a terminal call that returns an empty set).
  - Not changed, on purpose: `ListBuilder`, `MapBuilder` and `SortedMapBuilder` still encode "built" as a nil pointer and show the same misleading message for a zero value. They were outside this item; making them match is a small follow-up if you want the six builders consistent.

---

## Phase 3: Queue

- [x] 3.1 Make `Peek` not O(n) on repeated reuse. Chose (a): `normalize()` stores the reversed queue in an `atomic.Pointer[Queue[T]]` (`norm`) with a compare-and-swap, so concurrent first readers share one result or each make an equal one.
  - Measured (min of 5 interleaved rounds, N=10,000): `Peek` on a back-only queue 142 us -> 2.6 ns, 0 allocs; `Dequeue` 143 us -> 16 ns, 1 alloc (the new Queue); second pass of `Iterator` 195 us -> 29 us, 3 allocs instead of 10,002. The first read of a queue nobody has read is level (it also stores one 24 B queue). Front-populated paths (`DequeueFront`, `PeekFront`, `DrainFromSlice`, `Enqueue`, `EnqueueOne`) are level.
  - Cost: a queue that was read while back-only keeps a second copy of its elements until it is garbage. Documented on the type and in the README.
  - Report: `memory-bank/reports/bench-phase3-queue-ab-2026-11-17.txt`.
- [x] 3.2 `NewQueueOf` (and `NewQueue`) call `consFromSlice` directly: one allocation and 25% of the bytes fewer, 1.08-1.14x faster. A test checks the queue does not alias the caller's slice.
- [x] 3.3 `QueueBuilder`: done differently from the plan, on purpose. The plan said build the back cons-list directly. Both were built and measured; the builder now collects values in a slice and builds the **front** list once in `Queue()`.
  - Back list directly: building alone is 1.45-1.6x faster than the chosen design, but a built queue then pays a reversal on its first read (N more allocations, 24N more bytes, plus the cached copy). Build and drain is 15-18% slower and makes 50% more allocations (30,002 vs 20,013 at N=10,000).
  - Chosen design vs the old builder: build 1.3-1.45x faster, allocations halved, bytes +4.6% at N=10,000 (the slice), build and drain 1.54-1.63x faster. A queue is built to be consumed, which is why the front list won.
  - Its test (`BuiltQueueNeedsNoReversal`) pins that the first `Dequeue` of a built queue allocates only the new Queue.
- [x] 3.4 `QueueIterator` has one cursor (`node`, then `back` turned around once the walk reaches it) and one `Done` rule (`node == nil && back == nil`). On a queue with only a back list, `Iterator()` takes the cached reversal instead of reversing per iterator. `Next` after `Done` returns `-1, zero, false`.
- [x] 3.5 Different from the plan: instead of an assert on the size/front invariant, the redundant `size` field was deleted and `Len` is `front.length() + back.length()` (each cons node already stores its sublist length). The invariant can no longer be broken. The struct stays at 24 B (two list pointers and the atomic pointer; `size` is gone).
- Tests: `queue_model_test.go` (new) runs 6,000 random Enqueue / Dequeue / builder steps on a pool of versions against a slice model (reading the version a step came from after each step, and extending versions that are not the newest), plus iterator shapes (nil, empty, front only, back only, both, after dequeues), builder vs `Enqueue`, no aliasing of caller slices, a concurrent first read by 8 goroutines (run under `-race`), and allocation counts via `testing.AllocsPerRun` (exact, so not flaky). 35/35 mutations killed (cache not read or not stored, empty queue copied, wrong length, dropped front or back in `Enqueue`, `Dequeue` and `Iterator`, iterator transition skipped or kept, `Done` conditions, index not advanced, builder dropping or aliasing values, builder or `NewQueue` filling the wrong list). The mutation run found one real gap: no test built a queue with `NewQueue` and several values, so a `NewQueue` that filled the back list instead of the front survived. A pool entry for it was added. (An empty-queue iterator allocation test was added before the run, because reasoning about "normalize an empty queue" showed that mutation would otherwise be equivalent to the original by behavior.)

---

## Phase 4: List allocation (largest win, own sub-plan and PR)

Baseline problem: about 2.4 KB and 5.7 allocs per persistent `Append` at 100k.

- [ ] 4.0 Quick win found during 2.1: `List.cap()` (`list.go:59`) returns `1 << (depth*5)`, but a depth-d root holds 32^(d+1) values (a leaf holds 32, each level above multiplies by 32). It under-reports by 32x, so `append` adds a root level 32x too early and every append-built list is one level deeper than needed (measured on append loops: n=1024 has depth 2, minimal is 1; n=100000 has depth 4, minimal 3). One level less cut `Get` by 23-39% and persistent `Set` by 22-42% time and B/op with one fewer allocation when measured on `NewList`-built lists (see 2.1). Candidate fix: `1 << ((depth+1)*5)`. Check `prepend` (origin arithmetic uses `depth`), `slice` (contraction loop) and the iterator against a depth-minimal tree, and add the shape test from `list_bulk_test.go` for append-built and prepend-built lists. Do this before 4.1: it changes the baseline for the rest of Phase 4.
- [ ] 4.1 Design note first (short ADR): tail buffer (up to 32 elements) vs smaller nodes vs both. Include the effect on `Prepend`, `Slice`, `Set` and the iterator.
- [ ] 4.2 Implement the tail leaf in `List`: `Append` copies only the tail and pushes it into the trie when full.
- [ ] 4.3 Size branch and leaf nodes to occupancy instead of a fixed `[32]listNode` array. Evaluate a concrete node type in place of the `listNode` interface.
- [ ] 4.4 `Slice` on a slice-backed list should share via sub-slicing (`list.go:254-270`) instead of always copying.
- [ ] 4.5 `ListIterator`: advance within the current leaf instead of calling `seek()` per element, and avoid the fixed `[32]` stack.
- [ ] 4.6 Collapse the four near-duplicate slice/trie type-switch blocks (append, prepend, set, slice).
- [ ] 4.7 Gate: all List tests (including `-race`) pass, and a benchstat table shows the allocation drop. Target: at least a 10x reduction in B/op for `Append` at N=100k (to be confirmed against the baseline).

---

## Phase 5: Map hot path

- [ ] 5.1 In `mapValueNode.get` and the collision paths, compare the stored `keyHash` first and call `Equal` only on a hash match.
- [ ] 5.2 Store the key hash in array nodes and entries, so expansion at `immutable.go:308-314` doesn't rehash.
- [ ] 5.3 Hash quality (ints and strings are separate problems):
  - Ints: replace the identity `hashUint64` for values that fit in 32 bits with the SplitMix64-style finalizer the code already uses above 32 bits. Confirm with the sequential-vs-random benchmark (sequential is about 24% slower today).
  - Strings, default hasher (stays deterministic). Scratch benchmark in `/tmp/hashbench` (M-series arm64, mean ns/op, string keys of 8 / 16 / 64 / 1024 bytes; speed only, one repeated-byte string, distribution not measured):

    | hash | 8 B | 16 B | 64 B | 1 KB |
    |---|---|---|---|---|
    | current `31*h` | 5.4 | 8.0 | 41 | 1014 |
    | FNV-1a | 5.3 | 8.0 | 49 | 1303 |
    | `hash/maphash` | 6.0 | 5.3 | 6.5 | 88 |
    | rapidhash V3 (`go.dw1.io/rapidhash`, `HashString`) | 3.2 | 3.1 | 5.8 | 38.5 |

    The same benchmark as x86_64 under Rosetta (not native speed, ranking only) gave poly31 6.1 / 8.9 / 52 / 1221, maphash 12.3 / 12.1 / 13.4 / 150, rapidhash 5.8 / 5.9 / 9.0 / 56 for 8 B / 16 B / 64 B / 1 KB.
  - FNV-1a is dropped: it is no faster than the current hash.
  - rapidhash is the leading candidate. It is fastest at every size tested, and deterministic: "hello" hashed identically in two process runs. It is pure Go, MIT licensed, with no dependencies. Its amd64 assembly (`accum_amd64.s`) only implements `accumBlocks`, which is called from one place (`rapidhash.go:82`) and only for inputs over 448 bytes. Shorter keys, which is nearly every map key, run the same pure-Go code on every platform. So copying the generic path into the repo with the MIT notice loses nothing for typical keys, and avoids the first dependency. Only keys over 448 bytes would be slower than the library's amd64 path.
  - Still to do before adopting:
    - verify distribution quality of the folded 32-bit hash on realistic key sets (sequential strings like `key1..key100000`, UUIDs, paths), comparing collision counts and map depth against the current hash
    - re-run the string benchmarks in the repo's own Map benchmarks (Set/Get with string keys), not just the isolated hash
    - run the vendored copy's tests, and keep the upstream license header and commit hash in the file
    - fold 64 to 32 bits with `uint32(h) ^ uint32(h>>32)`
  - `hash/maphash` as an opt-in `Hasher`, not the default: speed is good, but `MakeSeed()` is random and a seed can't be recreated in another process, so a maphash default would make map iteration order differ on every run, which breaks the README's deterministic-iteration promise. As an opt-in constructor it gives hash-flooding resistance. A `K comparable` constructor could use `maphash.Comparable` and also cover struct keys that the default hasher rejects. Decide in open question 2.
  - `rapidhash.HashComparable` uses `reflect` (about 33 ns per call) and is not suitable for a hot path. xxh3 and `cespare/xxhash` are not benchmarked.
  - Default hashes stay deterministic. Iteration order will change after upgrade (see open question 2), so add a CHANGELOG note.
- [ ] 5.4 Typed fast paths in `defaultHasher`, so `any(key).(type)` is not evaluated on each call (per-K specialization at construction time).
- [ ] 5.5 Evaluate allocation per entry (inline single entries in bitmap-node slots). Do this only if 5.1-5.4 leave a measurable gap. It needs its own design note.
- [ ] 5.6 `SortedMap`: use `slices.BinarySearchFunc`, and compare the Comparer result with `> 0` or `< 0` rather than `== 1`.

---

## Phase 6: Housekeeping

- [ ] 6.1 Regenerate `default.pgo` after Phases 2-5 (README documents the procedure), and re-run the baseline.
- [ ] 6.2 Update the README benchmark tables and ADR claims (ADR-001 "53%", ADR-004 "~85%") from the final numbers.
- [ ] 6.3 Obsolete after 2.1: `BatchListBuilder` no longer produces oversize slice nodes. Check whether any code still tolerates a `listSliceNode` larger than `listSliceThreshold` (`append` and `prepend` convert at `size >= threshold`, which works for any size) and either delete that tolerance or leave a one-line comment; no test needed.
- [ ] 6.4 Repository hygiene (found 2026-11-17 during Phase 1): the compiled test binary `immutable.test` is tracked in git (added in `ce1f089`) and should be removed with `git rm` and ignored (`*.test` in `.gitignore`). The root also holds `baseline_phase4a.txt`, `bench_list_builders_after.txt`, `bench_map_builders_after.txt`, `bench_small_map_after.txt` and `baseline_performance.md`, all older single-shot results that the Phase 0 baseline supersedes: move them under `memory-bank/reports/archive/` or delete them. Needs owner approval (it deletes tracked files).
- [ ] 6.5 Run `/update-docs`, then `/review` again on the final diff.

---

## Order and dependencies

Do 1.1 (toolchain and CI) before the Phase 0 baseline run, so all numbers use one toolchain: 1.1 -> 0 -> rest of 1 -> 2 -> 3 -> 4 -> 5 -> 6. Phases 3 and 5 are independent of 4, and can run in parallel after 2 if wanted. Phase 4 and 5.5 each get a separate PR and a design note first.

## Risks

- Phase 4 and 5.3 change observable behavior (hash values, iteration order for maps with sequential keys). Decide the stability guarantee before starting.
- Removing or deprecating `BatchListBuilder`, `BatchMapBuilder` and `SortedBatchBuilder` is an API change. Prefer deprecate-and-alias over deletion.
- Queue memoization (3.1a) adds shared mutable state, so run `-race` and add a concurrent Peek test. Done: `TestQueueConcurrentReads`, and the full suite passes under `-race`.

## Open questions

1. ~~`go.mod` bump~~ Resolved: `go 1.26.0` stays, builds use 1.27.4 (see 1.1).
2. Hash values and iteration order (affects 5.3). The README promises that map iteration is deterministic (same keys give the same order on every run), but not a specific order. Proposed: allow changing the default hash for ints (finalizer) and strings (vendored rapidhash generic path, if it passes the distribution and Map benchmarks), keep both deterministic, and add a CHANGELOG note that iteration order may differ after the upgrade. Offer `hash/maphash` as an opt-in `Hasher`, not as the default, because its seed is random per process. Remaining choice for the owner: is deterministic iteration order across runs a feature to keep (the default), or is a Go-map-style random order acceptable in exchange for hash-flooding resistance? Pending the owner's answer.

   Dependency (proposed, owner can overrule): the owner chose "benchmark amd64 first; copy the generic path if it is close, otherwise decide on the dependency". The assembly only runs for inputs over 448 bytes, so the generic path is the same code for typical keys. Proposal: copy rapidhash's generic path into the repo with its MIT notice and add no dependency (details in 5.3).
3. ~~Batch builders~~ Resolved: keep them as efficiency tools, API may change (see Phase 2).
