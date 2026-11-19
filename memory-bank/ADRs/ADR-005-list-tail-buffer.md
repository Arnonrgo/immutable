# ADR-005: Tail Buffer for List.Append

## Status
Accepted (option A implemented in plan item 4.2 on 2026-11-17; the Options table below is the design-time model, the Measured results section has the numbers). Options B and D remain not adopted.

## Date
2026-11-17

## Context
A persistent `List.Append` copies the path from the root to the last leaf. At N = 100,000 the trie is 4 levels (depth 3) and one `Append` allocates 2,048 B in 5 allocations:

| Allocation | Bytes (T = int) |
|---|---|
| new leaf (32 values + occupancy mask) | 288 |
| 3 branch nodes (`d` + 32 interface children, 520 B, 576 B size class) | 3 x 576 |
| the `List` struct (root interface, origin, size) | 32 |

A list built by an `Append` loop therefore allocates about 2 KB per element, which is what the Phase 0 baseline measured (232 MiB for 100k appends). Almost all of it is garbage after the next `Append`, and 31 of every 32 appends only change the last leaf.

Constraints from the existing design:

- A `List` is persistent and "safe for concurrent use": a published list is never modified.
- `Prepend` and `Slice` work through `origin`, an offset of element 0 in the trie's index space. Leaves at the front can be partly filled; `occupied` masks and `deleteBefore`/`deleteAfter` keep references past the ends out of the tree.
- Lists of up to 32 values use a `listSliceNode` (ADR-004) and every operation type-switches on it.
- `BatchListBuilder` and `ListBuilder` use the same node types; the builders mutate nodes in place.

## Options

All sizes are Go allocator size classes, for `T = int`, N = 100k, per `Append` in an append loop. "Amortized" means one 32nd of the cost of pushing a leaf into the trie.

| Option | What changes | B/op | vs 2,048 B |
|---|---|---|---|
| Today | | 2,048 | 1x |
| **A** Tail buffer, copied on each `Append` | last up to 32 values live in a `[]T` outside the trie; `Append` copies the tail (about 135 B on average) and the `List` (64 B); a full tail is pushed into the trie as a leaf (63 B amortized: leaf 288 + 3 x 576 path, over 32) | about 262 | 7.8x |
| **B** Branches sized to occupancy | rightmost path at N=100k holds 4, about 16 and about 16 children instead of 32 each: 656 B instead of 1,728 B; `Append` alone: 288 + 656 + 32 | about 976 | 2.1x |
| **A + B** | | about 229 | 8.9x |
| **D** Shared tail with a CAS "used" counter | tail is a 32-slot array shared by versions; an `Append` to the version that owns the next free slot writes it in place, any other version copies | about 110-125 | 16-18x |
| **D + B** | | about 95 | 22x |

Reading the table:

- B alone is small, and with A it only trims the push (63 -> 30 B amortized, 13% of the total). It does not help `Set` on the interior of a big list, whose path nodes are full. It costs an extra allocation per node if the node becomes a header plus an array (Go has no flexible array members), or a family of node types if it does not.
- A is simple and keeps every published structure immutable.
- D is the only option that clears a 10x reduction, but it has two real costs. First, a published structure is no longer immutable: correctness depends on an atomic claim of the next slot, on the loser of a race copying, and on `Set`/`Slice`/`Prepend` never writing into a shared array. That is exactly the machinery that "a list is safe for concurrent use" must not break, and it needs its own race tests. Second, every list that has been appended to holds a 32-slot array (256 B for `int`), so a list of one value costs 288 B instead of about 8 B, which is a memory regression for the many-small-lists case that ADR-004 optimised.

## Decision
Adopt **A** now (plan 4.2), measure, and decide on B and D from the measurements.

1. The `List` keeps up to 32 of its last values in `tail []T`, outside the trie. The trie holds the rest. `Append` copies the tail (at most 32 values) and never touches the trie except when the tail is full: then the full tail becomes a leaf, pushed into the trie, and a new one-value tail starts.
2. The tail replaces `listSliceNode`. A list of up to 32 values is a tail with no trie. This removes the type switch from `Get`, `Set`, `Append`, `Prepend`, `Slice`, the iterator and `Contains`, which is most of plan item 4.6, and `listSliceNode` and its `toTrie` conversion go away.
3. **B is not part of this change.** Revisit it in 4.3 only with a measured gain on the final code. **D is not adopted**; it is recorded here so the 10x question can be answered with data.
4. The 4.7 gate (at least 10x fewer B/op for `Append` at N=100k) was not reachable with A or A + B on this model (7.8x and 8.9x), and A measured 7.3x (see Measured results). Accept it (Append allocates 256 B instead of about 1.9 KB, one tail copy and a clone) or take D with its costs above. That is a decision for the owner, not a default.

## Representation and invariants

```go
type List[T any] struct {
    root   listNode[T] // trie over the first size-len(tail) values; nil when there are none
    tail   []T         // the last 0..32 values, in order
    origin int         // trie index of element 0; 0 when root is nil
    size   int         // total number of values (trie + tail)
}
```

- **I1** `len(tail) <= 32`.
- **I2** The trie holds elements `[0, size-len(tail))` at trie indexes `origin+i`. When `root == nil` that count is 0 and `origin == 0`.
- **I3** The trie always ends on a leaf boundary: `(origin + size - len(tail)) % 32 == 0` whenever `root != nil`. A full tail is therefore exactly one leaf and is pushed without being split. The first leaf may be partly filled (`origin % 32 != 0` after `Prepend` or `Slice`), as it can be today.
- **I4** A published list is never modified, including its tail array: every operation that changes the tail makes a new one.

I3 is not enforced by the type system. 4.2 adds an internal `checkInvariants(l)` that the model tests (`TestListMatchesModel`, the depth and bulk tests) call after every operation, so a violation fails a test rather than corrupting a later `Append`.

## Effect on each operation

- **Get(i):** `i >= size-len(tail)` reads `tail[i-trieLen]`; otherwise `root.get(origin+i)`. One comparison replaces today's type assertion.
- **Append:** tail not full: allocate a tail of `len+1`, copy, store, clone the `List` (2 allocs, no trie work). Tail full: push it as a leaf at trie index `origin+trieLen` (path copy, `root` expanded by a level when the index is past `cap()`), then a one-value tail. With `root == nil` the pushed leaf becomes the root.
- **Prepend:** with a trie, unchanged (path copy, `origin` moves back, a new root level is added at `origin == 0`); I3 holds because `origin` falls by as much as `trieLen` grows. On a tail-only list it copies the tail, as the slice node does now; at 32 values the 33 values are split into a trie of leaves and a tail. Lists built by `Prepend` do not gain from the tail; a mirrored head buffer would be needed for that and is out of scope.
- **Set(i):** `i` in the tail: copy the tail and the `List`. `i` in the trie: the existing path copy (about 2 KB at 100k; this is inherent to a 32-way persistent trie and is not changed here).
- **Slice(start, end):** the trie part is cut with the existing `deleteBefore`/`deleteAfter` and root contraction. If `end` falls inside the trie, the elements from the last leaf boundary to `end` move into the new tail and the trie is cut at that boundary, so I3 holds (this is one extra copy of at most 32 values and is made by `Slice` itself, never deferred to a later `Append`). If the whole result is in the tail, it is a new tail.
- **Iterator:** walks the trie leaf by leaf, then the tail. Replaces the per-element `seek()` and the fixed `[32]listIteratorElem` stack (4.5); `Prev` and `Seek` follow the same split.
- **ListBuilder (mutable):** `Append` extends the tail in place, with amortized growth, and pushes it into the trie in place when it is full.
- **BatchListBuilder:** `List()` keeps wiring complete leaves into the trie without copying and copies the last leaf's values into the tail (at most 32 values). `NewList` builds the same shape: the last leaf is the tail.

## Consequences

### Positive
- `Append` allocates about 260 B instead of 2,048 B (modelled), and does not copy a path except once per 32 appends.
- A list of up to 32 values and a larger list share one code path for the tail, instead of a separate node type plus conversion.
- The zero value of `List` becomes usable. Today `var l List[int]; l.Append(1)` and `l.Prepend(1)` panic with a nil dereference (`l.root.depth()`), although the type documents the zero value as an empty list. With `root == nil` allowed, they work. 4.2 adds a test for it.

### Negative
- Every `List` is 24 bytes larger (a slice header): 64 B in the size class instead of 32 B. For a one-value list of `int` the total goes from 64 B in 3 allocations (32 B `List`, 24 B `listSliceNode`, 8 B array) to 72 B in 2 (64 B `List`, 8 B array), so a small list costs 8 B more and one allocation fewer. For lists of many values the 32 B is noise.
- `Set` and `Prepend` on large lists are unchanged; only the append path improves.
- Two more invariants (I2, I3) that tests, not types, enforce.
- `Slice` of a tail copies at most 32 values. Plan item 4.4 proposed sharing the slice node's array by sub-slicing, but `Slice` documents that references to inaccessible elements are removed so they can be collected. Sub-slicing would keep up to 31 hidden values alive per slice. Decision for 4.4: keep copying (it is at most 256 B for `int`) unless the documentation is deliberately weakened.

### Risks
- Mixed-origin trees: after `Prepend` and `Slice` the trie's first leaf is partial while its right end stays aligned. The random model test (Append, Prepend, Set, Slice over a pool of versions, including prepend-built and append-built lists) must run over the new representation and check I1-I3 each step.
- Tests that look inside the list (`list_bulk_test.go`, `immutable_test.go` "findLeaf" for the GC check, `list_depth_test.go`) assume `listSliceNode` and a root-only shape and have to move to the new shape, keeping what they pin (minimal depth, `origin == 0` after `Append`, no references outside the slice).

## Measured results (2026-11-17)

Old and new test binaries run alternately, minimum of 5 rounds, same machine under heavy load; allocation figures are exact. Full tables: `memory-bank/reports/bench-phase4-2-list-tail-ab-2026-11-17.txt`.

| Benchmark (T = int) | Before | After | Change |
|---|---|---|---|
| `Append` loop, 100,000 values: B per Append | 1,863 | 256 | 7.3x fewer (model: 7.8x) |
| same: allocs per Append | 4.69 | 2.11 | |
| same: time per build | 52.5 ms | 9.3 ms | 5.6x |
| `Append` loop, 10,000 / 1,000 values: time | | | 4.5x / 3.0x |
| `ListBuilder`, 100,000 values: time | 1.93 ms | 0.67 ms | 2.9x |
| `Append` onto 32 values | 2,080 B, 7 allocs | 360 B, 3 allocs | |
| `Append` onto 33 values | 896 B | 80 B | |
| `NewList` of 33 values | 1,184 B | 360 B | 3.0x faster |
| list of 1 value | 64 B, 3 allocs | 72 B, 2 allocs | +8 B |
| `Set` on 100,000 values | 2,048 B, 5 allocs | 2,079 B, 4 allocs | +31 B |
| `Get`, iteration, `Contains`, `Prepend`, `NewList`, `BatchListBuilder` | | | no change beyond noise |

The model said about 262 B per `Append`; the measurement is 256 B. The 10x gate in the plan is not met (7.3x).

## Implementation Notes
- 4.2 (done): `tail` replaces `listSliceNode`; `checkListInvariants` (I1-I3, trie slot occupancy, no values in unoccupied leaf slots or in the tail's spare capacity) runs in `requireListEquals`, the model test and the depth and bulk tests; `TestListZeroValueIsEmpty`; `TestListAppendAllocationContract` pins 2 allocations for an `Append` or `Set` that stays in the tail and 4 / 6 for a push at depth 1 / 3; `BenchmarkBaseline_List_Small` covers lists of 1, 8, 32 and 33 values. Mutation tests (32 mutants over the tail, push, `Slice`, iterator and batch builder code) all die except four that do not change behavior.
- Known small waste: pushing a full tail that needs a new root level allocates the new root and then copies it once more in `setLeaf` (one extra branch, once per 32^k appends). Not worth coupling `pushTail` to `setLeaf` ownership rules.
- 4.3 (branches sized to occupancy) and D are decided from the measured numbers above, not before.
