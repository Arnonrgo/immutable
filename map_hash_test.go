package immutable

import "testing"

// mapTrieStats describes the shape of a map's trie: how many values it holds,
// how many nodes it has, how deep the deepest value is, and how many collision
// nodes there are.
type mapTrieStats struct{ values, nodes, maxDepth, collisions int }

func trieStats(m *Map[int, int]) mapTrieStats {
	var s mapTrieStats
	var walk func(n mapNode[int, int], depth int)
	walk = func(n mapNode[int, int], depth int) {
		switch n := n.(type) {
		case *mapValueNode[int, int]:
			s.values++
			s.maxDepth = max(s.maxDepth, depth)
		case *mapHashCollisionNode[int, int]:
			s.collisions++
			s.values += len(n.entries)
			s.maxDepth = max(s.maxDepth, depth)
		case *mapBitmapIndexedNode[int, int]:
			s.nodes++
			for _, c := range n.nodes {
				walk(c, depth+1)
			}
		case *mapHashArrayNode[int, int]:
			s.nodes++
			for _, c := range n.nodes {
				if c != nil {
					walk(c, depth+1)
				}
			}
		case *mapArrayNode[int, int]:
			s.values += len(n.entries)
		}
	}
	walk(m.root, 0)
	return s
}

// TestDefaultIntHashKeepsDenseKeysCompact pins the decision of 5.3: the default
// hasher returns an int key below 2^32 unchanged. The trie takes 5 bits of the
// hash per level, so dense keys (indexes, ids, unix timestamps) fill every slot
// of a node before it needs a child: 1,000 consecutive keys take 33 nodes and
// two levels, where a mixed hash takes about 310 nodes and 2.65 levels on
// average (memory-bank/reports/bench-phase5-3-int-hash-2026-11-17.txt). A
// mixing finalizer would be slower to build and read for these keys.
// Keys above 2^32 are mixed, so they do not collide just because they share
// their low bits.
func TestDefaultIntHashKeepsDenseKeysCompact(t *testing.T) {
	const n = 1000
	for name, key := range map[string]func(i int) int{
		"sequential":  func(i int) int { return i },
		"timestamps":  func(i int) int { return 1_700_000_000 + i },
		"stride 7919": func(i int) int { return i * 7919 },
	} {
		t.Run(name, func(t *testing.T) {
			b := NewMapBuilder[int, int](nil)
			for i := 0; i < n; i++ {
				b.Set(key(i), i)
			}
			s := trieStats(b.Map())
			if s.values != n || s.collisions != 0 {
				t.Fatalf("%d values and %d collision nodes, want %d and 0", s.values, s.collisions, n)
			}
			if s.nodes > 40 || s.maxDepth != 2 {
				t.Errorf("%d nodes and depth %d, want at most 40 and 2 (a mixed hash takes about 310 and 4-5)", s.nodes, s.maxDepth)
			}
		})
	}

	t.Run("wide keys are mixed", func(t *testing.T) {
		b := NewMapBuilder[int, int](nil)
		for i := 0; i < n; i++ {
			b.Set(i<<40, i) // the same low 40 bits for every key
		}
		s := trieStats(b.Map())
		if s.values != n || s.collisions != 0 || s.maxDepth > 6 {
			t.Errorf("%d values, %d collision nodes and depth %d, want %d, 0 and at most 6", s.values, s.collisions, s.maxDepth, n)
		}
	})
}
