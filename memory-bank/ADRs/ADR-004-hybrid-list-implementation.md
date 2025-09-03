# ADR-004: Hybrid List Implementation for Small List Performance

## Status
Accepted

## Date
2024-07-23

## Context
The `List` implementation was based on a persistent trie structure (HAMT). While this provides excellent `O(log n)` performance for large lists, benchmarks revealed significant overhead for small lists, which are a very common use case. Performance for operations like `Append` and `Prepend` was ~30x slower than a native Go slice, with ~50x more allocations.

## Decision
Implement a hybrid data structure for the `List`, similar to the existing `Map` optimization.

1.  **Default to a Slice:** By default, a `List` will be backed by a simple `listSliceNode` which wraps a native Go slice (`[]T`).
2.  **Transparent Conversion:** When the number of elements in the slice-backed list exceeds a threshold (`listSliceThreshold = 32`), the underlying structure is transparently and automatically converted to the more scalable trie-based structure (`listBranchNode`).
3.  **Optimized Operations:** All `List` operations (`Append`, `Prepend`, `Set`, `Get`, `Slice`) are implemented with a type switch to use highly efficient slice-native operations when the list is small.

## Consequences

### Positive
- **Dramatic Performance Improvement:** Up to **2x faster** for common immutable operations (`Append`, `Prepend`) on small lists.
- **~85% Memory Reduction:** Drastically reduces allocation overhead for small lists, lessening GC pressure.
- **Best of Both Worlds:** Achieves performance close to native slices for the common case (small lists) while retaining `O(log n)` scalability for large lists.
- **API Unchanged:** This is a purely internal implementation detail. The public API of the `List` remains 100% backward compatible.
- **Pattern Consistency:** Aligns the `List` implementation strategy with the already-proven successful strategy of the `Map`.

### Negative
- **Builder Regression for Small Lists:** The `ListBuilder`, which uses mutable operations, sees a performance regression for lists that *remain under* the 32-element threshold. This is because each builder operation on the slice now involves a copy to maintain immutability guarantees between `List()` calls, whereas the trie could be mutated in-place more efficiently. This is deemed an acceptable trade-off as the builder's primary benefit is for constructing *large* lists, where it will still convert to and operate on the highly efficient trie.
- **Increased Code Complexity:** The `List` methods now require a type switch to handle the two underlying node types.

## Implementation Notes
- A new `listSliceNode[T]` struct was added.
- The `List` constructor `NewList[T]` now defaults to creating a `listSliceNode`.
- Core `List` methods were updated to handle the hybrid logic and the slice-to-trie conversion.
- Benchmarks were created to validate the performance improvements before and after the change. 