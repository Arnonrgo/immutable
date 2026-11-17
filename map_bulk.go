package immutable

// Bulk construction of a Map trie from a batch of entries.
//
// Inserting keys one at a time walks the trie from the root for every key and
// grows each branch node by appending (which allocates garbage), and a bitmap
// node is rebuilt as a hash array node when it fills. Building from the whole
// batch instead partitions the entries by the same 5-bit hash segments the trie
// uses, one level at a time, so every node is allocated once at its final size
// and no entry walks the tree.
//
// The result has the shape that calling Set for the same distinct keys would
// give. The only difference is the order of entries that share a full hash
// inside a collision node, which is first-set order here and can differ after a
// root array node has been expanded.

// mapBuildItem is one entry of the batch during a bulk build: the hash of its
// key and its position in the entries slice. Partitioning moves these 8 byte
// items instead of the entries.
type mapBuildItem struct {
	hash uint32
	idx  uint32
}

// mapBulkSmallGroup is the group size up to which a group is ordered by an
// insertion sort. A counting sort has to clear and walk 32 counters, which a
// group of a few items does not pay back.
const mapBulkSmallGroup = 12

// mapBulk holds the state of one bulk build.
type mapBulk[K, V any] struct {
	entries []mapEntry[K, V]
	hasher  Hasher[K]
	size    int // distinct keys seen so far while building
}

// buildMapRoot returns the root node holding the entries and the number of
// distinct keys. When a key occurs more than once the last value wins. The
// hasher must not be nil unless entries is empty.
func buildMapRoot[K, V any](entries []mapEntry[K, V], hasher Hasher[K]) (root mapNode[K, V], size int) {
	switch {
	case len(entries) == 0:
		return nil, 0
	case len(entries) <= maxArrayMapSize:
		return buildMapArray(entries, hasher)
	}

	assert(uint64(len(entries)) <= 1<<32-1, "immutable: too many entries to build a Map at once")

	// One allocation for the items and the scratch space the partitioning
	// ping-pongs with.
	buf := make([]mapBuildItem, 2*len(entries))
	items, scratch := buf[:len(entries):len(entries)], buf[len(entries):]
	for i := range entries {
		items[i] = mapBuildItem{hash: hasher.Hash(entries[i].key), idx: uint32(i)}
	}

	b := mapBulk[K, V]{entries: entries, hasher: hasher}
	root = b.node(items, scratch, 0)
	if b.size <= maxArrayMapSize {
		// More than maxArrayMapSize sets of at most that many distinct keys.
		// A map this small is a single array node, as it is when built by Set.
		return buildMapArray(entries, hasher)
	}
	return root, b.size
}

// buildMapArray returns an array node for entries, which must hold at most
// maxArrayMapSize distinct keys. Keys stay in first-set order and the last
// value for a key wins.
func buildMapArray[K, V any](entries []mapEntry[K, V], hasher Hasher[K]) (mapNode[K, V], int) {
	arr := &mapArrayNode[K, V]{entries: make([]mapEntry[K, V], 0, min(len(entries), maxArrayMapSize))}
	for _, e := range entries {
		if i := arr.indexOf(e.key, hasher); i >= 0 {
			arr.entries[i].value = e.value
		} else {
			arr.entries = append(arr.entries, e)
		}
	}
	return arr, len(arr.entries)
}

// node builds the subtree for items, which all agree on the hash segments
// above shift. scratch is the same length as items. Both are overwritten.
func (b *mapBulk[K, V]) node(items, scratch []mapBuildItem, shift uint) mapNode[K, V] {
	if len(items) == 1 {
		b.size++
		e := &b.entries[items[0].idx]
		return newMapValueNode(items[0].hash, e.key, e.value)
	}
	if sameHash(items) {
		return b.leaf(items)
	}

	// The hashes differ, so some segment at or below shift separates them.
	sorted, other := sortByHashSegment(items, scratch, shift)

	runs := 1
	for i := 1; i < len(sorted); i++ {
		if hashSegment(sorted[i].hash, shift) != hashSegment(sorted[i-1].hash, shift) {
			runs++
		}
	}

	// Same cut-over as inserting one key at a time: a bitmap indexed node
	// takes up to maxBitmapIndexedSize+1 children.
	if runs > maxBitmapIndexedSize+1 {
		n := &mapHashArrayNode[K, V]{count: uint(runs)}
		for lo := 0; lo < len(sorted); {
			hi := runEnd(sorted, lo, shift)
			n.nodes[hashSegment(sorted[lo].hash, shift)] = b.node(sorted[lo:hi], other[lo:hi], shift+mapNodeBits)
			lo = hi
		}
		return n
	}

	n := &mapBitmapIndexedNode[K, V]{nodes: make([]mapNode[K, V], 0, runs)}
	for lo := 0; lo < len(sorted); {
		hi := runEnd(sorted, lo, shift)
		n.bitmap |= 1 << hashSegment(sorted[lo].hash, shift)
		n.nodes = append(n.nodes, b.node(sorted[lo:hi], other[lo:hi], shift+mapNodeBits))
		lo = hi
	}
	return n
}

// leaf returns the leaf for two or more items with the same hash: a value node
// when they are all the same key, otherwise a collision node. Later values
// overwrite earlier ones and keys stay in first-set order.
func (b *mapBulk[K, V]) leaf(items []mapBuildItem) mapNode[K, V] {
	hash := items[0].hash
	var entries []mapEntry[K, V]
	for _, it := range items {
		e := b.entries[it.idx]
		i := 0
		for i < len(entries) && !b.hasher.Equal(entries[i].key, e.key) {
			i++
		}
		if i < len(entries) {
			entries[i].value = e.value
		} else {
			entries = append(entries, e)
		}
	}
	b.size += len(entries)
	if len(entries) == 1 {
		return newMapValueNode(hash, entries[0].key, entries[0].value)
	}
	return &mapHashCollisionNode[K, V]{keyHash: hash, entries: entries}
}

func hashSegment(hash uint32, shift uint) uint32 {
	return (hash >> shift) & mapNodeMask
}

// sameHash reports whether all items have the same hash. It stops at the first
// difference, which for a group of different hashes is usually the second item.
func sameHash(items []mapBuildItem) bool {
	for i := 1; i < len(items); i++ {
		if items[i].hash != items[0].hash {
			return false
		}
	}
	return true
}

// runEnd returns the end of the run of items starting at lo that share the hash
// segment at shift. items must be ordered by that segment.
func runEnd(items []mapBuildItem, lo int, shift uint) int {
	seg := hashSegment(items[lo].hash, shift)
	hi := lo + 1
	for hi < len(items) && hashSegment(items[hi].hash, shift) == seg {
		hi++
	}
	return hi
}

// sortByHashSegment stably orders items by their hash segment at shift. It
// returns the slice holding the ordered items and the other slice, so the
// caller can use the pair for the next level. scratch must be as long as items.
func sortByHashSegment(items, scratch []mapBuildItem, shift uint) (sorted, other []mapBuildItem) {
	if len(items) <= mapBulkSmallGroup {
		for i := 1; i < len(items); i++ {
			it := items[i]
			seg := hashSegment(it.hash, shift)
			j := i
			for j > 0 && hashSegment(items[j-1].hash, shift) > seg {
				items[j] = items[j-1]
				j--
			}
			items[j] = it
		}
		return items, scratch
	}

	var next [mapNodeSize]int
	for _, it := range items {
		next[hashSegment(it.hash, shift)]++
	}
	// All in one segment: already ordered, nothing to move.
	if next[hashSegment(items[0].hash, shift)] == len(items) {
		return items, scratch
	}
	pos := 0
	for seg := range next {
		next[seg], pos = pos, pos+next[seg]
	}
	for _, it := range items {
		seg := hashSegment(it.hash, shift)
		scratch[next[seg]] = it
		next[seg]++
	}
	return scratch, items
}
