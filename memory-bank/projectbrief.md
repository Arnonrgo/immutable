# Project Brief: Immutable Data Structures Library
*Version: 1.1*
*Created: 2024-12-19*
*Last Updated: 2024-12-24*

## Project Overview
A high-performance* Go library providing immutable collection types (List, Map, SortedMap, Set, SortedSet) with efficient copy-on-write semantics and structural sharing optimizations. Designed for safe concurrent read access and minimal memory overhead through pointer-based array sharing.

(*) considering immutability

## Core Requirements
- ✅ Immutable List with append/prepend/set operations
- ✅ Immutable Map with hash-based key lookup
- ✅ Immutable SortedMap with ordered iteration
- ✅ Immutable Set and SortedSet collections
- ✅ Builder patterns for efficient batch operations
- ✅ Thread-safe read operations without locks
- ✅ Generic type safety with Go 1.18+ generics
- ✅ Pointer-based structural sharing optimization


## Scope
- Core immutable data structures (List, Map, SortedMap, Set, SortedSet)
- Builder patterns for efficient construction
- Iterator interfaces for traversal
- Comprehensive benchmarking suite with memory analysis
- Pointer-based array optimization eliminating 53% memory allocation bottleneck
- Thread-safe concurrent read operations
- Large value scaling optimization (exponential benefits with value size)

### Out of Scope
- Mutable variants of data structures
- Serialization/deserialization



*This document serves as the foundation for the project and informs all other memory files.* 
