package immutable

import (
	"fmt"
	"math/bits"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// Tests for the bulk map build (map_bulk.go) and BatchMapBuilder. The build is
// checked three ways: against a Go map (contents), against the trie that
// calling Set one key at a time produces (shape), and against the structural
// rules every node must obey (checkMapShape), so a bug that both builders
// shared would still be caught.

// modHasher hashes an int to k % mod, so many distinct keys share a hash.
type modHasher struct{ mod int }

func (h modHasher) Hash(k int) uint32   { return uint32(k % h.mod) }
func (h modHasher) Equal(a, b int) bool { return a == b }

// shiftHasher puts the key in the top bits of the hash, so keys share every
// low hash segment and the trie gets a chain of single-child nodes.
type shiftHasher struct{ shift uint }

func (h shiftHasher) Hash(k int) uint32   { return uint32(k) << h.shift }
func (h shiftHasher) Equal(a, b int) bool { return a == b }

// foldHasher treats strings that differ only by case as the same key.
type foldHasher struct{}

func (foldHasher) Hash(k string) uint32   { return hashString(strings.ToLower(k)) }
func (foldHasher) Equal(a, b string) bool { return strings.EqualFold(a, b) }

type kv struct{ k, v int }

// oracle applies sets in order to a Go map: last write wins.
func oracle(sets []kv) map[int]int {
	want := make(map[int]int, len(sets))
	for _, s := range sets {
		want[s.k] = s.v
	}
	return want
}

// buildBatch and buildIncremental build the same sets with BatchMapBuilder and
// with MapBuilder.
func buildBatch(h Hasher[int], sets []kv) *Map[int, int] {
	b := NewBatchMapBuilder[int, int](h, 0)
	for _, s := range sets {
		b.Set(s.k, s.v)
	}
	return b.Map()
}

func buildIncremental(h Hasher[int], sets []kv) *Map[int, int] {
	b := NewMapBuilder[int, int](h)
	for _, s := range sets {
		b.Set(s.k, s.v)
	}
	return b.Map()
}

// checkMapShape fails the test if the trie under m breaks a structural rule,
// and checks m.Len() against the number of entries found.
func checkMapShape(t *testing.T, m *Map[int, int]) {
	t.Helper()
	checkMapShapeOpt(t, m, true)
}

// checkMapShapeOpt is checkMapShape; collapsed says whether a branch with a
// single value child is an error. Bulk builds never make one, but Delete leaves
// them behind, so a map that has seen deletes is checked with collapsed false.
func checkMapShapeOpt(t *testing.T, m *Map[int, int], collapsed bool) {
	t.Helper()
	if m.root == nil {
		if m.Len() != 0 {
			t.Fatalf("nil root but Len() = %d", m.Len())
		}
		return
	}
	if got := checkMapNode(t, m.root, 0, 0, m.hasher, true, collapsed); got != m.Len() {
		t.Fatalf("trie holds %d entries but Len() = %d", got, m.Len())
	}
}

// checkMapNode returns the number of entries under n. prefix holds the hash
// segments chosen on the way down, in the bits below shift.
func checkMapNode(t *testing.T, n mapNode[int, int], shift uint, prefix uint32, h Hasher[int], root, collapsed bool) int {
	t.Helper()
	atPrefix := func(hash uint32) bool {
		return shift >= 32 || hash&(1<<shift-1) == prefix
	}
	switch n := n.(type) {
	case *mapArrayNode[int, int]:
		if !root {
			t.Fatalf("array node below the root (shift %d)", shift)
		}
		if len(n.entries) < 1 || len(n.entries) > maxArrayMapSize {
			t.Fatalf("array node with %d entries", len(n.entries))
		}
		seen := map[int]bool{}
		for _, e := range n.entries {
			if h.Hash(e.key) != e.keyHash {
				t.Fatalf("array node entry for key %d stores hash %#x, hasher gives %#x", e.key, e.keyHash, h.Hash(e.key))
			}
			if seen[e.key] {
				t.Fatalf("array node repeats key %d", e.key)
			}
			seen[e.key] = true
		}
		return len(n.entries)
	case *mapValueNode[int, int]:
		if h.Hash(n.key) != n.keyHash {
			t.Fatalf("value node for key %d stores hash %#x, hasher gives %#x", n.key, n.keyHash, h.Hash(n.key))
		}
		if !atPrefix(n.keyHash) {
			t.Fatalf("value node for key %d (hash %#x) sits on the wrong path (shift %d, prefix %#x)", n.key, n.keyHash, shift, prefix)
		}
		return 1
	case *mapHashCollisionNode[int, int]:
		if len(n.entries) < 2 {
			t.Fatalf("collision node with %d entries", len(n.entries))
		}
		if !atPrefix(n.keyHash) {
			t.Fatalf("collision node (hash %#x) sits on the wrong path (shift %d, prefix %#x)", n.keyHash, shift, prefix)
		}
		seen := map[int]bool{}
		for _, e := range n.entries {
			if h.Hash(e.key) != n.keyHash {
				t.Fatalf("collision node %#x holds key %d with hash %#x", n.keyHash, e.key, h.Hash(e.key))
			}
			if seen[e.key] {
				t.Fatalf("collision node repeats key %d", e.key)
			}
			seen[e.key] = true
		}
		return len(n.entries)
	case *mapBitmapIndexedNode[int, int]:
		if bits.OnesCount32(n.bitmap) != len(n.nodes) {
			t.Fatalf("bitmap %#x has %d bits but %d children", n.bitmap, bits.OnesCount32(n.bitmap), len(n.nodes))
		}
		if len(n.nodes) < 1 || len(n.nodes) > maxBitmapIndexedSize+1 {
			t.Fatalf("bitmap node with %d children", len(n.nodes))
		}
		if shift > 30 {
			t.Fatalf("branch node below the last hash segment (shift %d)", shift)
		}
		total, i := 0, 0
		for seg := uint32(0); seg < mapNodeSize; seg++ {
			if n.bitmap&(1<<seg) == 0 {
				continue
			}
			child := n.nodes[i]
			i++
			if collapsed && len(n.nodes) == 1 {
				switch child.(type) {
				case *mapBitmapIndexedNode[int, int], *mapHashArrayNode[int, int]:
				default:
					t.Fatalf("single-child bitmap node holds a %T, which should have been placed directly", child)
				}
			}
			total += checkMapNode(t, child, shift+mapNodeBits, prefix|seg<<shift, h, false, collapsed)
		}
		return total
	case *mapHashArrayNode[int, int]:
		if shift > 30 {
			t.Fatalf("branch node below the last hash segment (shift %d)", shift)
		}
		set, total := 0, 0
		for seg, child := range n.nodes {
			if child == nil {
				continue
			}
			set++
			total += checkMapNode(t, child, shift+mapNodeBits, prefix|uint32(seg)<<shift, h, false, collapsed)
		}
		if uint(set) != n.count {
			t.Fatalf("hash array node count %d, but %d children are set", n.count, set)
		}
		if set <= maxBitmapIndexedSize+1 {
			t.Fatalf("hash array node with only %d children", set)
		}
		return total
	case nil:
		t.Fatalf("nil node inside the trie (shift %d)", shift)
	default:
		t.Fatalf("unexpected node type %T", n)
	}
	return 0
}

// sameMapShape reports whether two tries have the same node types, bitmaps and
// entries. Entries of array and collision nodes are compared as sets.
func sameMapShape(a, b mapNode[int, int]) bool {
	sameEntries := func(x, y []mapEntry[int, int]) bool {
		if len(x) != len(y) {
			return false
		}
		for _, e := range x {
			if !slices.Contains(y, e) {
				return false
			}
		}
		return true
	}
	sameArrayEntries := func(x, y []mapArrayEntry[int, int]) bool {
		if len(x) != len(y) {
			return false
		}
		for _, e := range x {
			if !slices.Contains(y, e) {
				return false
			}
		}
		return true
	}
	switch a := a.(type) {
	case *mapArrayNode[int, int]:
		b, ok := b.(*mapArrayNode[int, int])
		return ok && sameArrayEntries(a.entries, b.entries)
	case *mapValueNode[int, int]:
		b, ok := b.(*mapValueNode[int, int])
		return ok && *a == *b
	case *mapHashCollisionNode[int, int]:
		b, ok := b.(*mapHashCollisionNode[int, int])
		return ok && a.keyHash == b.keyHash && sameEntries(a.entries, b.entries)
	case *mapBitmapIndexedNode[int, int]:
		b, ok := b.(*mapBitmapIndexedNode[int, int])
		if !ok || a.bitmap != b.bitmap || len(a.nodes) != len(b.nodes) {
			return false
		}
		for i := range a.nodes {
			if !sameMapShape(a.nodes[i], b.nodes[i]) {
				return false
			}
		}
		return true
	case *mapHashArrayNode[int, int]:
		b, ok := b.(*mapHashArrayNode[int, int])
		if !ok || a.count != b.count {
			return false
		}
		for i := range a.nodes {
			if (a.nodes[i] == nil) != (b.nodes[i] == nil) {
				return false
			}
			if a.nodes[i] != nil && !sameMapShape(a.nodes[i], b.nodes[i]) {
				return false
			}
		}
		return true
	}
	return a == nil && b == nil
}

// requireMapEquals checks m against want through Len, Get, absent keys and the
// iterator.
func requireMapEquals(t *testing.T, m *Map[int, int], want map[int]int) {
	t.Helper()
	if m.Len() != len(want) {
		t.Fatalf("Len() = %d, want %d", m.Len(), len(want))
	}
	for k, v := range want {
		if got, ok := m.Get(k); !ok || got != v {
			t.Fatalf("Get(%d) = %d, %v; want %d, true", k, got, ok, v)
		}
	}
	seen := map[int]bool{}
	for itr := m.Iterator(); !itr.Done(); {
		k, v, _ := itr.Next()
		if seen[k] {
			t.Fatalf("iterator returned key %d twice", k)
		}
		seen[k] = true
		if want[k] != v {
			t.Fatalf("iterator returned %d=%d, want %d", k, v, want[k])
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("iterator returned %d entries, want %d", len(seen), len(want))
	}
}

// randomSets returns n sets with keys drawn from [0, space).
func randomSets(r *rand.Rand, n, space int) []kv {
	sets := make([]kv, n)
	for i := range sets {
		sets[i] = kv{k: r.Intn(space), v: i}
	}
	return sets
}

// TestBulkMapMatchesIncremental builds the same sets both ways, with the
// default hasher and with hashers that force collisions and deep chains, and
// requires the same contents and the same trie shape.
func TestBulkMapMatchesIncremental(t *testing.T) {
	hashers := map[string]func() Hasher[int]{
		"default":        func() Hasher[int] { return nil },
		"collide3":       func() Hasher[int] { return modHasher{3} },
		"collide100":     func() Hasher[int] { return modHasher{100} },
		"collideAll":     func() Hasher[int] { return modHasher{1} },
		"topBitsShift27": func() Hasher[int] { return shiftHasher{27} },
		"topBitsShift24": func() Hasher[int] { return shiftHasher{24} },
	}
	sizes := []int{0, 1, 2, 7, 8, 9, 10, 24, 25, 26, 27, 32, 33, 64, 200, 1000, 3000}
	for name, newHasher := range hashers {
		for _, n := range sizes {
			for _, space := range []int{3, n, 1 << 20} {
				if space < 1 {
					continue
				}
				if name == "topBitsShift27" && space > 32 || name == "topBitsShift24" && space > 256 {
					space = 32
					if name == "topBitsShift24" {
						space = 256
					}
				}
				t.Run(fmt.Sprintf("%s/n%d/space%d", name, n, space), func(t *testing.T) {
					sets := randomSets(rand.New(rand.NewSource(int64(n*131+space))), n, space)
					bulk := buildBatch(newHasher(), sets)
					inc := buildIncremental(newHasher(), sets)

					requireMapEquals(t, bulk, oracle(sets))
					checkMapShape(t, bulk)
					checkMapShape(t, inc) // keeps the checker honest
					if !sameMapShape(bulk.root, inc.root) {
						t.Fatalf("bulk-built trie differs in shape from the Set-built one")
					}
				})
			}
		}
	}
}

// TestBulkMapShape pins the shape for inputs where the right answer is known
// without the incremental builder.
func TestBulkMapShape(t *testing.T) {
	seq := func(n int) []kv {
		sets := make([]kv, n)
		for i := range sets {
			sets[i] = kv{i, i}
		}
		return sets
	}
	repeated := func(distinct, times int) []kv {
		var sets []kv
		for i := 0; i < times; i++ {
			for k := 0; k < distinct; k++ {
				sets = append(sets, kv{k, i})
			}
		}
		return sets
	}
	spread := func(n, stride int) []kv {
		sets := make([]kv, n)
		for i := range sets {
			sets[i] = kv{i * stride, i}
		}
		return sets
	}

	tests := []struct {
		name string
		h    Hasher[int]
		sets []kv
		root string // %T of the root
		len  int
	}{
		{"empty", nil, nil, "<nil>", 0},
		{"one", nil, seq(1), "*immutable.mapArrayNode[int,int]", 1},
		{"eight", nil, seq(8), "*immutable.mapArrayNode[int,int]", 8},
		{"nine", nil, seq(9), "*immutable.mapBitmapIndexedNode[int,int]", 9},
		{"twentyFive", nil, seq(25), "*immutable.mapBitmapIndexedNode[int,int]", 25},
		{"twentySix", nil, seq(26), "*immutable.mapHashArrayNode[int,int]", 26},
		{"manySetsFewKeys", nil, repeated(5, 100), "*immutable.mapArrayNode[int,int]", 5},
		{"manySetsEightKeys", nil, repeated(8, 50), "*immutable.mapArrayNode[int,int]", 8},
		{"manySetsNineKeys", nil, repeated(9, 50), "*immutable.mapBitmapIndexedNode[int,int]", 9},
		{"sharedLowSegments", nil, spread(20, 1<<10), "*immutable.mapBitmapIndexedNode[int,int]", 20},
		{"allSameHashFew", modHasher{1}, seq(6), "*immutable.mapArrayNode[int,int]", 6},
		{"allSameHashMany", modHasher{1}, seq(20), "*immutable.mapHashCollisionNode[int,int]", 20},
		{"sameHashOneKey", modHasher{1}, repeated(1, 40), "*immutable.mapArrayNode[int,int]", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := buildBatch(tt.h, tt.sets)
			if got := fmt.Sprintf("%T", m.root); got != tt.root {
				t.Fatalf("root is %s, want %s", got, tt.root)
			}
			if m.Len() != tt.len {
				t.Fatalf("Len() = %d, want %d", m.Len(), tt.len)
			}
			requireMapEquals(t, m, oracle(tt.sets))
			checkMapShape(t, m)
		})
	}

	t.Run("sharedLowSegmentsChain", func(t *testing.T) {
		// 20 keys that are multiples of 1<<10 share hash segments 0 and 1, so the
		// root and its child each have a single child.
		m := buildBatch(nil, spread(20, 1<<10))
		node, depth := m.root, 0
		for {
			b, ok := node.(*mapBitmapIndexedNode[int, int])
			if !ok || len(b.nodes) != 1 {
				break
			}
			node, depth = b.nodes[0], depth+1
		}
		if depth != 2 {
			t.Fatalf("chain of %d single-child nodes, want 2", depth)
		}
	})
}

// TestBulkMapLastWriteWins checks that a repeated key keeps its last value in
// every part of the build: the array node, a value node, and a collision node.
func TestBulkMapLastWriteWins(t *testing.T) {
	for _, h := range []struct {
		name string
		h    Hasher[int]
	}{{"default", nil}, {"collideAll", modHasher{1}}, {"collide4", modHasher{4}}} {
		for _, n := range []int{3, 12, 100} {
			t.Run(fmt.Sprintf("%s/n%d", h.name, n), func(t *testing.T) {
				var sets []kv
				for round := 0; round < 5; round++ {
					for k := 0; k < n; k++ {
						sets = append(sets, kv{k, round*1000 + k})
					}
				}
				m := buildBatch(h.h, sets)
				for k := 0; k < n; k++ {
					if v, ok := m.Get(k); !ok || v != 4000+k {
						t.Fatalf("Get(%d) = %d, %v; want %d", k, v, ok, 4000+k)
					}
				}
				checkMapShape(t, m)
			})
		}
	}
}

// TestBulkMapUsesHasherEquality checks that keys the Hasher calls equal are one
// key, not just keys that are ==.
func TestBulkMapUsesHasherEquality(t *testing.T) {
	for _, n := range []int{3, 9, 40} { // array path, and trie path with leaf groups
		t.Run(fmt.Sprintf("n%d", n), func(t *testing.T) {
			b := NewBatchMapBuilder[string, int](foldHasher{}, 0)
			for i := 0; i < n; i++ {
				b.Set(fmt.Sprintf("key%d", i), i)
			}
			for i := 0; i < n; i++ {
				b.Set(fmt.Sprintf("KEY%d", i), 1000+i)
			}
			m := b.Map()
			if m.Len() != n {
				t.Fatalf("Len() = %d, want %d: keys that differ only by case should be one key", m.Len(), n)
			}
			for i := 0; i < n; i++ {
				for _, k := range []string{fmt.Sprintf("key%d", i), fmt.Sprintf("Key%d", i)} {
					if v, ok := m.Get(k); !ok || v != 1000+i {
						t.Fatalf("Get(%q) = %d, %v; want %d", k, v, ok, 1000+i)
					}
				}
			}
		})
	}
}

// TestBulkMapPersistentOperations runs Set and Delete on a bulk-built map and
// on its older versions, so every node type the build can produce is used by
// the code that mutates it.
func TestBulkMapPersistentOperations(t *testing.T) {
	for _, h := range []struct {
		name string
		h    func() Hasher[int]
	}{
		{"default", func() Hasher[int] { return nil }},
		{"collide7", func() Hasher[int] { return modHasher{7} }},
		{"topBits", func() Hasher[int] { return shiftHasher{24} }},
	} {
		for _, n := range []int{5, 9, 30, 400, 2500} {
			t.Run(fmt.Sprintf("%s/n%d", h.name, n), func(t *testing.T) {
				space := 4 * n
				if h.name == "topBits" {
					space = 200
				}
				r := rand.New(rand.NewSource(int64(n)))
				sets := randomSets(r, n, space)
				base := buildBatch(h.h(), sets)
				want := oracle(sets)
				baseWant := maps2Clone(want)

				m := base
				for step := 0; step < 3*n; step++ {
					k := r.Intn(space + 10)
					if r.Intn(2) == 0 {
						m = m.Set(k, -step)
						want[k] = -step
					} else {
						m = m.Delete(k)
						delete(want, k)
					}
				}
				requireMapEquals(t, m, want)
				requireMapEquals(t, base, baseWant) // the original is untouched
				checkMapShape(t, base)
			})
		}
	}
}

func maps2Clone(m map[int]int) map[int]int {
	c := make(map[int]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// TestSortByHashSegment checks the partitioning step on its own: the output is
// ordered by segment, stable, a permutation of the input, and the second slice
// is the other buffer.
func TestSortByHashSegment(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	for _, n := range []int{0, 1, 2, 11, 12, 13, 14, 40, 500} {
		for _, shift := range []uint{0, 5, 25, 30} {
			for _, mode := range []string{"random", "oneSegment", "twoSegments"} {
				t.Run(fmt.Sprintf("n%d/shift%d/%s", n, shift, mode), func(t *testing.T) {
					items := make([]mapBuildItem, n)
					for i := range items {
						hash := r.Uint32()
						switch mode {
						case "oneSegment":
							hash &^= mapNodeMask << shift
							hash |= 7 << shift
						case "twoSegments":
							hash &^= mapNodeMask << shift
							hash |= uint32(r.Intn(2)*9) << shift
						}
						items[i] = mapBuildItem{hash: hash, idx: uint32(i)}
					}
					input := slices.Clone(items)
					scratch := make([]mapBuildItem, n)
					sorted, other := sortByHashSegment(items, scratch, shift)

					if len(sorted) != n || len(other) != n {
						t.Fatalf("lengths %d and %d, want %d", len(sorted), len(other), n)
					}
					if n > 0 && (&sorted[0] == &other[0]) {
						t.Fatal("sorted and other are the same buffer")
					}
					for i := 1; i < n; i++ {
						a, b := sorted[i-1], sorted[i]
						sa, sb := hashSegment(a.hash, shift), hashSegment(b.hash, shift)
						if sa > sb || sa == sb && a.idx > b.idx {
							t.Fatalf("items %d and %d are out of order: %+v then %+v", i-1, i, a, b)
						}
					}
					got := slices.Clone(sorted)
					slices.SortFunc(got, func(a, b mapBuildItem) int { return int(a.idx) - int(b.idx) })
					if !slices.Equal(got, input) {
						t.Fatal("output is not a permutation of the input")
					}
				})
			}
		}
	}
}

func TestBatchMapBuilderLifecycle(t *testing.T) {
	t.Run("LenCountsSets", func(t *testing.T) {
		b := NewBatchMapBuilder[int, int](nil, 4)
		if b.Len() != 0 {
			t.Fatalf("Len() = %d on a new builder", b.Len())
		}
		for i := 0; i < 20; i++ {
			b.Set(i%5, i)
		}
		if b.Len() != 20 {
			t.Fatalf("Len() = %d, want 20 (one per Set)", b.Len())
		}
		if got := b.Map().Len(); got != 5 {
			t.Fatalf("map Len() = %d, want 5 distinct keys", got)
		}
	})

	t.Run("PanicsAfterMap", func(t *testing.T) {
		b := NewBatchMapBuilder[int, int](nil, 0)
		b.Set(1, 1)
		b.Map()
		mustPanic(t, "Set", func() { b.Set(2, 2) })
		mustPanic(t, "SetMap", func() { b.SetMap(map[int]int{3: 3}) })
		mustPanic(t, "Len", func() { b.Len() })
		mustPanic(t, "Map", func() { b.Map() })
	})

	t.Run("ResetReuses", func(t *testing.T) {
		b := NewBatchMapBuilder[int, int](nil, 0)
		for i := 0; i < 50; i++ {
			b.Set(i, i)
		}
		b.Reset()
		if b.Len() != 0 {
			t.Fatalf("Len() = %d after Reset", b.Len())
		}
		b.Set(7, 70)
		m := b.Map()
		requireMapEquals(t, m, map[int]int{7: 70})

		b.Reset() // also revives a builder that already produced its map
		for i := 0; i < 20; i++ {
			b.Set(i, -i)
		}
		m2 := b.Map()
		want := map[int]int{}
		for i := 0; i < 20; i++ {
			want[i] = -i
		}
		requireMapEquals(t, m2, want)
		requireMapEquals(t, m, map[int]int{7: 70}) // the first map is not affected
	})

	t.Run("EmptyMapIsUsable", func(t *testing.T) {
		m := NewBatchMapBuilder[int, int](nil, 0).Map()
		if m.Len() != 0 {
			t.Fatalf("Len() = %d", m.Len())
		}
		if _, ok := m.Get(1); ok {
			t.Fatal("Get on an empty map found a key")
		}
		m = m.Set(1, 2) // the hasher is chosen by the first Set, as for NewMap
		if v, ok := m.Get(1); !ok || v != 2 {
			t.Fatalf("Get(1) = %d, %v", v, ok)
		}
	})

	t.Run("HasherChosenAtSet", func(t *testing.T) {
		// A key type with no default hasher must fail at the Set that cannot be
		// hashed, as MapBuilder does, not later in Map().
		type unhashable struct{ a, b int }
		b := NewBatchMapBuilder[unhashable, int](nil, 0)
		mustPanic(t, "Set with no hasher", func() { b.Set(unhashable{1, 2}, 1) })
	})

	t.Run("MapOwnsItsEntries", func(t *testing.T) {
		// Reusing the builder after Map() must not change the map that was built.
		b := NewBatchMapBuilder[int, int](nil, 0)
		for i := 0; i < 5; i++ {
			b.Set(i, i)
		}
		m := b.Map()
		b.Reset()
		for i := 0; i < 5; i++ {
			b.Set(i, 100+i)
		}
		for i := 0; i < 5; i++ {
			if v, _ := m.Get(i); v != i {
				t.Fatalf("Get(%d) = %d after the builder was reused, want %d", i, v, i)
			}
		}
	})

	t.Run("FlushIsHarmless", func(t *testing.T) {
		b := NewBatchMapBuilder[int, int](nil, 3)
		for i := 0; i < 10; i++ {
			b.Set(i, i)
			b.Flush()
		}
		if got := b.Map().Len(); got != 10 {
			t.Fatalf("Len() = %d, want 10", got)
		}
	})
}

// TestMapShapeUnderRandomSets applies random Set and Delete calls over a small
// key range, which keeps the map crossing the array-node limit (8 entries) in
// both directions, to a persistent map and to a MapBuilder. After every call the
// trie must keep its structural rules (including that an array node stores the
// hash of each key) and agree with a Go map.
func TestMapShapeUnderRandomSets(t *testing.T) {
	hashers := map[string]func() Hasher[int]{
		"default":  func() Hasher[int] { return nil },
		"collide3": func() Hasher[int] { return modHasher{3} },
		"collide1": func() Hasher[int] { return modHasher{1} },
		"high":     func() Hasher[int] { return shiftHasher{27} },
	}
	for name, mk := range hashers {
		t.Run(name, func(t *testing.T) {
			rng := splitmix(len(name))
			m := NewMap[int, int](mk())
			b := NewMapBuilder[int, int](mk())
			want := map[int]int{}
			for step := 0; step < 3000; step++ {
				k, v := rng.intn(24), rng.intn(1000)
				if rng.intn(3) == 0 {
					m = m.Delete(k)
					b.Delete(k)
					delete(want, k)
				} else {
					m = m.Set(k, v)
					b.Set(k, v)
					want[k] = v
				}
				checkMapShapeOpt(t, m, false) // Delete leaves single-child branches
				checkMapShapeOpt(t, b.m, false)
				if m.Len() != len(want) || b.Len() != len(want) {
					t.Fatalf("step %d: Len() = %d (builder %d), want %d", step, m.Len(), b.Len(), len(want))
				}
			}
			requireMapEquals(t, m, want)
			requireMapEquals(t, b.m, want)
		})
	}
}

// countingHasher hashes an int to itself and counts its calls.
type countingHasher struct{ hashes, equals int }

func (h *countingHasher) Hash(k int) uint32 { h.hashes++; return uint32(k) }
func (h *countingHasher) Equal(a, b int) bool {
	h.equals++
	return a == b
}

// TestMapArrayNodeHashesOnce pins what 5.1 and 5.2 bought. A small map keeps the
// hash of each entry, so a lookup compares hashes first and calls Equal only on
// a match, and growing past the array limit moves the entries into the trie
// with the stored hashes instead of hashing every key again.
func TestMapArrayNodeHashesOnce(t *testing.T) {
	h := &countingHasher{}
	m := NewMap[int, int](h)
	for i := 0; i < maxArrayMapSize; i++ {
		before := h.hashes
		m = m.Set(i, i)
		if got := h.hashes - before; got != 1 {
			t.Fatalf("Set #%d hashed %d times, want 1", i+1, got)
		}
	}
	if _, ok := m.root.(*mapArrayNode[int, int]); !ok {
		t.Fatalf("a map of %d entries is a %T, want an array node", m.Len(), m.root)
	}

	// The next Set is the one that leaves the array node.
	h.hashes, h.equals = 0, 0
	m = m.Set(maxArrayMapSize, 0)
	if _, ok := m.root.(*mapArrayNode[int, int]); ok {
		t.Fatalf("a map of %d entries is still an array node", m.Len())
	}
	if h.hashes != 1 {
		t.Errorf("the Set that expands the array node hashed %d times, want 1 (the new key)", h.hashes)
	}
	checkMapShape(t, m)

	// Lookups in an array node: one hash, and Equal only for the entry whose hash
	// matches.
	small := NewMap[int, int](h)
	for i := 0; i < maxArrayMapSize; i++ {
		small = small.Set(100+i, i)
	}
	h.hashes, h.equals = 0, 0
	if v, ok := small.Get(103); !ok || v != 3 {
		t.Fatalf("Get(103) = %d, %v, want 3, true", v, ok)
	}
	if h.hashes != 1 || h.equals != 1 {
		t.Errorf("Get of a present key: %d Hash and %d Equal calls, want 1 and 1", h.hashes, h.equals)
	}
	h.hashes, h.equals = 0, 0
	if _, ok := small.Get(7); ok {
		t.Fatal("Get(7) found a key that is not in the map")
	}
	if h.hashes != 1 || h.equals != 0 {
		t.Errorf("Get of an absent key: %d Hash and %d Equal calls, want 1 and 0 (no hash matches)", h.hashes, h.equals)
	}
	h.hashes, h.equals = 0, 0
	sinkAny = small.Delete(7)
	if h.hashes != 1 || h.equals != 0 {
		t.Errorf("Delete of an absent key: %d Hash and %d Equal calls, want 1 and 0", h.hashes, h.equals)
	}

	// Expansion builds the trie in place: the persistent path copy for each of the
	// eight entries would cost several times this many allocations. An ordinary
	// Set into a small map costs 4 (see the first loop below).
	full := NewMap[int, int](nil)
	for i := 0; i < maxArrayMapSize; i++ {
		if got := testing.AllocsPerRun(20, func() { sinkAny = full.Set(-1, 1) }); got > 4 {
			t.Errorf("Set into a map of %d entries: %v allocs, want at most 4", i, got)
		}
		full = full.Set(i*1000003+7, i)
	}
	if got := testing.AllocsPerRun(20, func() { sinkAny = full.Set(-1, 1) }); got > 17 {
		t.Errorf("the Set that expands an array node: %v allocs, want at most 17", got)
	}
}
