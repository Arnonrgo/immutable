package immutable

import (
	"cmp"
	"slices"
)

// Bulk construction of a SortedMap B+tree from a batch of entries.
//
// Inserting keys one at a time searches a path through the tree for every key,
// shifts entries inside a leaf, and splits nodes as they fill. Building from
// the whole batch instead sorts the entries once and then cuts them into
// leaves, and the leaves into branches, so every node is allocated once at its
// final size and no key is searched for.

// sortedBuildEntry is an entry collected by a SortedBatchBuilder. seq is the
// position of the Set call that added it, which lets the sort be unstable (it
// is much faster than the stable one) and still keep equal keys in the order
// they were set.
type sortedBuildEntry[K, V any] struct {
	key   K
	value V
	seq   uint32
}

// buildSortedMapRoot returns the root node holding the entries and the number
// of distinct keys. It sorts and compacts entries in place, so the caller must
// not use the slice afterwards. When keys compare equal the last entry wins,
// key and value, as it does when Set is called for each entry in order. The
// comparer must not be nil unless entries is empty.
func buildSortedMapRoot[K, V any](entries []sortedBuildEntry[K, V], comparer Comparer[K]) (root sortedMapNode[K, V], size int) {
	if len(entries) == 0 {
		return nil, 0
	}
	assert(uint64(len(entries)) <= 1<<32-1, "immutable: too many entries to build a SortedMap at once")

	// Input that is already in strictly increasing order, such as keys read
	// from a sorted source, needs no sort and has no duplicates. Anything else
	// fails the check within a few entries, so the check is cheap to make.
	if !strictlyIncreasing(entries, comparer) {
		// Ordering equal keys by seq makes the last of each run the entry that
		// was set last, so it is the one that wins.
		slices.SortFunc(entries, func(a, b sortedBuildEntry[K, V]) int {
			if c := comparer.Compare(a.key, b.key); c != 0 {
				return c
			}
			return cmp.Compare(a.seq, b.seq)
		})
		entries = compactSortedEntries(entries, comparer)
	}

	nodes := sortedLeaves(entries)
	for len(nodes) > 1 {
		nodes = sortedBranches(nodes)
	}
	return nodes[0], len(entries)
}

// strictlyIncreasing reports whether every key is greater than the one before.
func strictlyIncreasing[K, V any](entries []sortedBuildEntry[K, V], comparer Comparer[K]) bool {
	for i := 1; i < len(entries); i++ {
		if comparer.Compare(entries[i-1].key, entries[i].key) >= 0 {
			return false
		}
	}
	return true
}

// compactSortedEntries removes all but the last entry of each run of equal keys
// from the sorted entries and returns the shortened slice.
func compactSortedEntries[K, V any](entries []sortedBuildEntry[K, V], comparer Comparer[K]) []sortedBuildEntry[K, V] {
	out := entries[:1]
	for _, e := range entries[1:] {
		if last := &out[len(out)-1]; comparer.Compare(last.key, e.key) == 0 {
			*last = e
		} else {
			out = append(out, e)
		}
	}
	clear(entries[len(out):]) // drop references held past the end
	return out
}

// sortedLeaves cuts the sorted entries into leaf nodes. The entries are spread
// evenly over the fewest leaves that can hold them, so no leaf is nearly empty
// and, unless the count is a multiple of the leaf size, none is full.
func sortedLeaves[K, V any](entries []sortedBuildEntry[K, V]) []sortedMapNode[K, V] {
	n := len(entries)
	k := (n + sortedMapNodeSize - 1) / sortedMapNodeSize
	leaves := make([]sortedMapNode[K, V], k)
	for i := range leaves {
		lo, hi := i*n/k, (i+1)*n/k
		leaf := &sortedMapLeafNode[K, V]{entries: make([]mapEntry[K, V], hi-lo)}
		for j, e := range entries[lo:hi] {
			leaf.entries[j] = mapEntry[K, V]{key: e.key, value: e.value}
		}
		leaves[i] = leaf
	}
	return leaves
}

// sortedBranches joins nodes into the branch nodes one level above them, spread
// evenly like the leaves. len(nodes) must be more than one, so every branch gets
// at least two children.
func sortedBranches[K, V any](nodes []sortedMapNode[K, V]) []sortedMapNode[K, V] {
	n := len(nodes)
	k := (n + sortedMapNodeSize - 1) / sortedMapNodeSize
	branches := make([]sortedMapNode[K, V], k)
	for i := range branches {
		lo, hi := i*n/k, (i+1)*n/k
		elems := make([]sortedMapBranchElem[K, V], hi-lo)
		for j, child := range nodes[lo:hi] {
			elems[j] = sortedMapBranchElem[K, V]{key: child.minKey(), node: child}
		}
		branches[i] = &sortedMapBranchNode[K, V]{elems: elems}
	}
	return branches
}
