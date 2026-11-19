package immutable

import (
	"cmp"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// Tests for the bulk sorted map build (sorted_bulk.go), SortedBatchBuilder and
// BatchSortedSetBuilder. The build is checked against a model (contents and
// order), against the map that calling Set one key at a time produces, and
// against the structural rules every B+tree node must obey (checkSortedMapShape),
// so a bug that both builders shared would still be caught.

// sortedKV is one Set call.
type sortedKV struct{ k, v int }

// sortedOracle returns the distinct keys of sets in ascending order and the
// value each key ends up with, which is the value of its last Set.
func sortedOracle(sets []sortedKV) (keys []int, want map[int]int) {
	want = make(map[int]int, len(sets))
	for _, s := range sets {
		want[s.k] = s.v
	}
	for k := range want {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys, want
}

func buildSortedBatch(sets []sortedKV) *SortedMap[int, int] {
	b := NewSortedBatchBuilder[int, int](nil, 0)
	for _, s := range sets {
		b.Set(s.k, s.v)
	}
	return b.SortedMap()
}

func buildSortedIncremental(sets []sortedKV) *SortedMap[int, int] {
	m := NewSortedMap[int, int](nil)
	for _, s := range sets {
		m = m.set(s.k, s.v, true)
	}
	return m
}

// checkSortedMapShape verifies the B+tree rules: size matches, keys strictly
// increase across the whole tree, node sizes are within bounds, every branch
// key is the minimum key of its child, and all leaves are at the same depth.
// When bulk is set it also checks what the bulk build promises on top: the
// fewest possible leaves and no branch with a single child.
func checkSortedMapShape[K, V any](t *testing.T, m *SortedMap[K, V], bulk bool) {
	t.Helper()
	if m.root == nil {
		if m.size != 0 {
			t.Fatalf("empty tree with size %d", m.size)
		}
		return
	}
	c := m.comparer
	leafDepth, leaves, count := -1, 0, 0
	var last *K
	var walk func(n sortedMapNode[K, V], depth int)
	walk = func(n sortedMapNode[K, V], depth int) {
		switch n := n.(type) {
		case *sortedMapLeafNode[K, V]:
			if len(n.entries) == 0 || len(n.entries) > sortedMapNodeSize {
				t.Fatalf("leaf with %d entries", len(n.entries))
			}
			if leafDepth == -1 {
				leafDepth = depth
			} else if leafDepth != depth {
				t.Fatalf("leaves at depths %d and %d", leafDepth, depth)
			}
			leaves++
			for i := range n.entries {
				if last != nil && c.Compare(*last, n.entries[i].key) >= 0 {
					t.Fatalf("key %v does not come after %v", n.entries[i].key, *last)
				}
				last = &n.entries[i].key
				count++
			}
		case *sortedMapBranchNode[K, V]:
			if len(n.elems) == 0 || len(n.elems) > sortedMapNodeSize {
				t.Fatalf("branch with %d children", len(n.elems))
			}
			if bulk && len(n.elems) < 2 {
				t.Fatalf("bulk-built branch with %d child", len(n.elems))
			}
			for _, e := range n.elems {
				if c.Compare(e.key, e.node.minKey()) != 0 {
					t.Fatalf("branch key %v, but the child starts at %v", e.key, e.node.minKey())
				}
				walk(e.node, depth+1)
			}
		default:
			t.Fatalf("unexpected node type %T", n)
		}
	}
	walk(m.root, 0)
	if count != m.size {
		t.Fatalf("size %d, but the tree holds %d entries", m.size, count)
	}
	if want := (count + sortedMapNodeSize - 1) / sortedMapNodeSize; bulk && leaves != want {
		t.Fatalf("%d leaves for %d entries, want %d", leaves, count, want)
	}
}

// requireSortedMapEquals checks Len, Get, forward and backward iteration and
// Seek of m against the sorted distinct keys and their values.
func requireSortedMapEquals(t *testing.T, m *SortedMap[int, int], keys []int, want map[int]int) {
	t.Helper()
	if m.Len() != len(keys) {
		t.Fatalf("Len() = %d, want %d", m.Len(), len(keys))
	}
	i := 0
	for itr := m.Iterator(); !itr.Done(); i++ {
		k, v, ok := itr.Next()
		if !ok || i >= len(keys) || k != keys[i] || v != want[k] {
			t.Fatalf("forward[%d] = %d, %d, %v; want %d, %d", i, k, v, ok, safeAt(keys, i), want[safeAt(keys, i)])
		}
	}
	if i != len(keys) {
		t.Fatalf("forward iteration gave %d entries, want %d", i, len(keys))
	}
	i = len(keys) - 1
	itr := m.Iterator()
	itr.Last()
	for ; !itr.Done(); i-- {
		k, v, ok := itr.Prev()
		if !ok || i < 0 || k != keys[i] || v != want[k] {
			t.Fatalf("backward[%d] = %d, %d, %v; want %d", i, k, v, ok, safeAt(keys, i))
		}
	}
	if i != -1 {
		t.Fatalf("backward iteration stopped %d entries short", i+1)
	}
	for k, v := range want {
		if got, ok := m.Get(k); !ok || got != v {
			t.Fatalf("Get(%d) = %d, %v; want %d, true", k, got, ok, v)
		}
	}
	// Seek lands on the key itself, or on the next one when the key is absent.
	for _, probe := range seekProbes(keys) {
		itr := m.Iterator()
		itr.Seek(probe)
		idx, _ := slices.BinarySearch(keys, probe)
		k, _, ok := itr.Next()
		if idx == len(keys) {
			if ok {
				t.Fatalf("Seek(%d) found %d past the last key", probe, k)
			}
		} else if !ok || k != keys[idx] {
			t.Fatalf("Seek(%d) = %d, %v; want %d", probe, k, ok, keys[idx])
		}
	}
	if len(keys) > 0 {
		if _, ok := m.Get(keys[0] - 1); ok {
			t.Fatalf("Get(%d) found a key below the first", keys[0]-1)
		}
		if _, ok := m.Get(keys[len(keys)-1] + 1); ok {
			t.Fatalf("Get(%d) found a key above the last", keys[len(keys)-1]+1)
		}
	}
}

func safeAt(keys []int, i int) int {
	if i < 0 || i >= len(keys) {
		return -1
	}
	return keys[i]
}

// seekProbes returns every key, and the values just below and above them, for
// a bounded number of keys spread over the whole range.
func seekProbes(keys []int) []int {
	step := max(1, len(keys)/200)
	var probes []int
	for i := 0; i < len(keys); i += step {
		probes = append(probes, keys[i]-1, keys[i], keys[i]+1)
	}
	if len(keys) > 0 {
		probes = append(probes, keys[len(keys)-1], keys[len(keys)-1]+1)
	}
	return probes
}

// sortedInputs returns Set sequences of n entries in several orders. The
// values are the position in the sequence, so a wrong winner among equal keys
// is visible.
func sortedInputs(r *rand.Rand, n int) map[string][]sortedKV {
	mk := func(key func(i int) int) []sortedKV {
		sets := make([]sortedKV, n)
		for i := range sets {
			sets[i] = sortedKV{k: key(i), v: i}
		}
		return sets
	}
	perm := r.Perm(n)
	return map[string][]sortedKV{
		"ascending":       mk(func(i int) int { return i * 2 }),
		"descending":      mk(func(i int) int { return (n - i) * 2 }),
		"random":          mk(func(i int) int { return perm[i] * 2 }),
		"random dups":     mk(func(int) int { return r.Intn(n/4 + 1) }),
		"ascending dups":  mk(func(i int) int { return i / 3 }),
		"all equal":       mk(func(int) int { return 7 }),
		"sawtooth":        mk(func(i int) int { return i % 37 }),
		"negative random": mk(func(i int) int { return perm[i] - n/2 }),
	}
}

// TestBulkSortedMatchesIncremental builds the same sets with SortedBatchBuilder,
// with Set one key at a time and with a model, across sizes that sit on every
// node-capacity boundary of a B+tree with 32-wide nodes, and requires the same
// contents and a valid tree.
func TestBulkSortedMatchesIncremental(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	sizes := []int{0, 1, 2, 3, 31, 32, 33, 48, 63, 64, 65, 96, 97, 1000, 1023, 1024, 1025, 1056, 1057, 3000, 32*32*32 + 1}
	for _, n := range sizes {
		for name, sets := range sortedInputs(r, n) {
			t.Run(fmt.Sprintf("%s/%d", name, n), func(t *testing.T) {
				keys, want := sortedOracle(sets)
				batch := buildSortedBatch(sets)
				checkSortedMapShape(t, batch, true)
				requireSortedMapEquals(t, batch, keys, want)

				inc := buildSortedIncremental(sets)
				checkSortedMapShape(t, inc, false)
				requireSortedMapEquals(t, inc, keys, want)
			})
		}
	}
}

// TestBulkSortedShape pins the tree the bulk build produces.
func TestBulkSortedShape(t *testing.T) {
	build := func(n int) *SortedMap[int, int] {
		sets := make([]sortedKV, n)
		for i := range sets {
			sets[i] = sortedKV{i, i}
		}
		return buildSortedBatch(sets)
	}
	depth := func(m *SortedMap[int, int]) int {
		d := 0
		for n := m.root; ; d++ {
			br, ok := n.(*sortedMapBranchNode[int, int])
			if !ok {
				return d
			}
			n = br.elems[0].node
		}
	}
	for _, c := range []struct{ n, depth, rootKids int }{
		{1, 0, 0}, {32, 0, 0}, // one leaf is the root
		{33, 1, 2}, {64, 1, 2}, {65, 1, 3}, {1024, 1, 32}, // one level of branches
		{1025, 2, 2}, {32 * 32 * 32, 2, 32}, // two levels
		{32*32*32 + 1, 3, 2},
	} {
		m := build(c.n)
		if got := depth(m); got != c.depth {
			t.Errorf("n=%d: depth %d, want %d", c.n, got, c.depth)
		}
		if br, ok := m.root.(*sortedMapBranchNode[int, int]); ok && len(br.elems) != c.rootKids {
			t.Errorf("n=%d: root has %d children, want %d", c.n, len(br.elems), c.rootKids)
		}
	}

	// Entries are spread evenly, so the smallest leaf holds at least half of
	// the largest, and a map of 33 entries is not a full leaf and a stub.
	m := build(33)
	br := m.root.(*sortedMapBranchNode[int, int])
	sizes := []int{len(br.elems[0].node.(*sortedMapLeafNode[int, int]).entries), len(br.elems[1].node.(*sortedMapLeafNode[int, int]).entries)}
	if !slices.Equal(sizes, []int{16, 17}) {
		t.Errorf("33 entries are split %v, want [16 17]", sizes)
	}
}

// caseFold compares strings ignoring case, so "a" and "A" are the same key.
type caseFold struct{}

func (caseFold) Compare(a, b string) int {
	return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
}

// reverseInts compares ints in descending order.
type reverseInts struct{}

func (reverseInts) Compare(a, b int) int { return cmp.Compare(b, a) }

type point struct{ x, y int }

// pointOrder orders points by x, then y. point is not cmp.Ordered.
type pointOrder struct{}

func (pointOrder) Compare(a, b point) int {
	if c := cmp.Compare(a.x, b.x); c != 0 {
		return c
	}
	return cmp.Compare(a.y, b.y)
}

type strKV struct {
	k string
	v int
}

func entriesOf[K, V any](m *SortedMap[K, V]) (keys []K, vals []V) {
	for itr := m.Iterator(); !itr.Done(); {
		k, v, _ := itr.Next()
		keys = append(keys, k)
		vals = append(vals, v)
	}
	return keys, vals
}

// TestSortedBatchBuilderHonorsComparer checks that the supplied Comparer, not
// the natural order of the keys, decides order and equality.
func TestSortedBatchBuilderHonorsComparer(t *testing.T) {
	t.Run("Reverse", func(t *testing.T) {
		for _, n := range []int{5, 40, 1500} {
			b := NewSortedBatchBuilder[int, int](reverseInts{}, 0)
			inc := NewSortedMap[int, int](reverseInts{})
			r := rand.New(rand.NewSource(2))
			for i := 0; i < n; i++ {
				k := r.Intn(n)
				b.Set(k, i)
				inc = inc.set(k, i, true)
			}
			m := b.SortedMap()
			checkSortedMapShape(t, m, true)
			gk, gv := entriesOf(m)
			wk, wv := entriesOf(inc)
			if !slices.Equal(gk, wk) || !slices.Equal(gv, wv) {
				t.Fatalf("n=%d: entries differ from Set one at a time\n got %v\nwant %v", n, gk, wk)
			}
			if !slices.IsSortedFunc(gk, func(a, b int) int { return cmp.Compare(b, a) }) {
				t.Fatalf("n=%d: keys are not in descending order: %v", n, gk)
			}
			for _, k := range gk {
				if _, ok := m.Get(k); !ok {
					t.Fatalf("n=%d: Get(%d) failed", n, k)
				}
			}
		}
	})

	// Keys that the comparer calls equal are one key, and the last Set wins,
	// key included, exactly as with Set one key at a time.
	t.Run("EqualButDistinctKeys", func(t *testing.T) {
		b := NewSortedBatchBuilder[string, int](caseFold{}, 0)
		for i, k := range []string{"B", "a", "A", "c", "b"} {
			b.Set(k, i)
		}
		m := b.SortedMap()
		gk, gv := entriesOf(m)
		if want := []string{"A", "b", "c"}; !slices.Equal(gk, want) {
			t.Errorf("keys %v, want %v", gk, want)
		}
		if want := []int{2, 4, 3}; !slices.Equal(gv, want) {
			t.Errorf("values %v, want %v", gv, want)
		}
		if got, ok := m.Get("a"); !ok || got != 2 {
			t.Errorf(`Get("a") = %d, %v; want 2, true`, got, ok)
		}
	})

	t.Run("EqualButDistinctKeysMany", func(t *testing.T) {
		r := rand.New(rand.NewSource(3))
		var sets []strKV
		for i := 0; i < 600; i++ {
			k := fmt.Sprintf("key%02d", r.Intn(20))
			if r.Intn(2) == 0 {
				k = strings.ToUpper(k)
			}
			sets = append(sets, strKV{k, i})
		}
		b := NewSortedBatchBuilder[string, int](caseFold{}, 0)
		inc := NewSortedMap[string, int](caseFold{})
		for _, s := range sets {
			b.Set(s.k, s.v)
			inc = inc.set(s.k, s.v, true)
		}
		gk, gv := entriesOf(b.SortedMap())
		wk, wv := entriesOf(inc)
		if !slices.Equal(gk, wk) || !slices.Equal(gv, wv) {
			t.Fatalf("entries differ from Set one at a time\n got %v %v\nwant %v %v", gk, gv, wk, wv)
		}
	})

	// A key type that is not cmp.Ordered works with a Comparer.
	t.Run("StructKeys", func(t *testing.T) {
		b := NewSortedBatchBuilder[point, string](pointOrder{}, 0)
		var want []point
		for x := 9; x >= 0; x-- {
			for y := 0; y < 5; y++ {
				p := point{x, y}
				b.Set(p, fmt.Sprint(p))
				want = append(want, p)
			}
		}
		m := b.SortedMap()
		checkSortedMapShape(t, m, true)
		slices.SortFunc(want, pointOrder{}.Compare)
		gk, _ := entriesOf(m)
		if !slices.Equal(gk, want) {
			t.Fatalf("got %v\nwant %v", gk, want)
		}
		if got, ok := m.Get(point{3, 2}); !ok || got != "{3 2}" {
			t.Errorf("Get({3 2}) = %q, %v", got, ok)
		}
	})

	t.Run("DefaultComparerForStrings", func(t *testing.T) {
		b := NewSortedBatchBuilder[string, int](nil, 0)
		for i, k := range []string{"pear", "apple", "fig", "apple"} {
			b.Set(k, i)
		}
		gk, gv := entriesOf(b.SortedMap())
		if !slices.Equal(gk, []string{"apple", "fig", "pear"}) || !slices.Equal(gv, []int{3, 2, 0}) {
			t.Errorf("got %v %v", gk, gv)
		}
	})

	t.Run("NoDefaultComparerPanicsAtSet", func(t *testing.T) {
		b := NewSortedBatchBuilder[point, int](nil, 0)
		mustPanic(t, "Set with a key that has no default comparer", func() { b.Set(point{1, 2}, 0) })
	})
}

// TestSortedBatchBuilderLastWriteWins covers the case the old maintainSort
// option got wrong: it kept the first of two Sets of one key.
func TestSortedBatchBuilderLastWriteWins(t *testing.T) {
	for _, n := range []int{2, 10, 100, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := NewSortedBatchBuilder[int, string](nil, 0)
			for round := 0; round < 3; round++ {
				for k := 0; k < n; k++ {
					b.Set(k, fmt.Sprintf("round%d", round))
				}
			}
			m := b.SortedMap()
			if m.Len() != n {
				t.Fatalf("Len() = %d, want %d", m.Len(), n)
			}
			for k := 0; k < n; k++ {
				if got, _ := m.Get(k); got != "round2" {
					t.Fatalf("Get(%d) = %q, want round2", k, got)
				}
			}
		})
	}

	t.Run("AcrossFlush", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, string](nil, 2)
		b.Set(1, "first")
		b.Flush()
		b.Set(1, "second")
		b.Flush()
		if got, _ := b.SortedMap().Get(1); got != "second" {
			t.Errorf("Get(1) = %q, want second", got)
		}
	})
}

// TestBulkSortedPersistentOperations applies a long run of persistent Set and
// Delete calls to bulk-built maps, which exercises splits and removals in trees
// built with full nodes, and checks that the starting map did not change.
func TestBulkSortedPersistentOperations(t *testing.T) {
	for _, n := range []int{0, 1, 32, 33, 100, 1024, 1025, 5000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			r := rand.New(rand.NewSource(int64(n) + 4))
			var sets []sortedKV
			for i := 0; i < n; i++ {
				sets = append(sets, sortedKV{k: r.Intn(3*n/2 + 1), v: i})
			}
			start := buildSortedBatch(sets)
			startKeys, startWant := sortedOracle(sets)

			_, want := sortedOracle(sets)
			m := start
			for op := 0; op < 3000; op++ {
				k := r.Intn(2*n + 2)
				if r.Intn(3) == 0 {
					m = m.Delete(k)
					delete(want, k)
				} else {
					m = m.Set(k, -op)
					want[k] = -op
				}
				if op%250 == 249 {
					keys := make([]int, 0, len(want))
					for k := range want {
						keys = append(keys, k)
					}
					slices.Sort(keys)
					checkSortedMapShape(t, m, false)
					requireSortedMapEquals(t, m, keys, want)
				}
			}
			checkSortedMapShape(t, start, true)
			requireSortedMapEquals(t, start, startKeys, startWant)
		})
	}
}

// TestSortedBatchBuilderLifecycle covers the builder states.
func TestSortedBatchBuilderLifecycle(t *testing.T) {
	t.Run("LenCountsSets", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 0)
		if b.Len() != 0 {
			t.Errorf("Len() = %d, want 0", b.Len())
		}
		b.Set(1, 1)
		b.Set(1, 2)
		b.Set(2, 3)
		if b.Len() != 3 {
			t.Errorf("Len() = %d, want 3 (an upper bound: key 1 was set twice)", b.Len())
		}
		if got := b.SortedMap().Len(); got != 2 {
			t.Errorf("SortedMap().Len() = %d, want 2", got)
		}
	})

	t.Run("PanicsAfterSortedMap", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 0)
		b.Set(1, 1)
		_ = b.SortedMap()
		mustPanic(t, "Set", func() { b.Set(2, 2) })
		mustPanic(t, "Len", func() { b.Len() })
		mustPanic(t, "second SortedMap()", func() { b.SortedMap() })
	})

	t.Run("ResetReuses", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 0)
		for i := 0; i < 100; i++ {
			b.Set(i, i)
		}
		first := b.SortedMap()

		b.Reset()
		if b.Len() != 0 {
			t.Fatalf("Len() after Reset = %d, want 0", b.Len())
		}
		for i := 0; i < 100; i++ {
			b.Set(i+1000, -i)
		}
		second := b.SortedMap()

		// The first map must not see the second build.
		keys := make([]int, 100)
		want := make(map[int]int, 100)
		for i := range keys {
			keys[i], want[i] = i, i
		}
		requireSortedMapEquals(t, first, keys, want)
		checkSortedMapShape(t, first, true)
		if got, _ := second.Get(1005); got != -5 {
			t.Errorf("second.Get(1005) = %d, want -5", got)
		}
		if _, ok := second.Get(5); ok {
			t.Error("second map holds a key of the first build")
		}
	})

	t.Run("ResetDiscardsPendingEntries", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 0)
		for i := 0; i < 50; i++ {
			b.Set(i, i)
		}
		b.Reset() // without SortedMap() first
		if b.Len() != 0 {
			t.Fatalf("Len() after Reset = %d, want 0", b.Len())
		}
		b.Set(100, 1)
		m := b.SortedMap()
		if m.Len() != 1 {
			t.Errorf("Len() = %d, want 1: entries set before Reset leaked into the map", m.Len())
		}
		if _, ok := m.Get(7); ok {
			t.Error("map holds a key set before Reset")
		}
	})

	t.Run("ResetKeepsComparer", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 0)
		b.Set(1, 1)
		_ = b.SortedMap()
		b.Reset()
		b.Set(3, 3)
		b.Set(2, 2)
		gk, _ := entriesOf(b.SortedMap())
		if !slices.Equal(gk, []int{2, 3}) {
			t.Errorf("keys %v, want [2 3]", gk)
		}
	})

	t.Run("EmptyMapIsUsable", func(t *testing.T) {
		m := NewSortedBatchBuilder[int, int](nil, 0).SortedMap()
		if m == nil || m.Len() != 0 {
			t.Fatalf("empty builder gave %v", m)
		}
		if _, ok := m.Get(1); ok {
			t.Error("Get on an empty map found a key")
		}
		if !m.Iterator().Done() {
			t.Error("iterator over an empty map is not done")
		}
		m2 := m.Set(5, 50)
		if got, ok := m2.Get(5); !ok || got != 50 || m2.Len() != 1 {
			t.Errorf("Set on an empty bulk-built map: %d, %v, len %d", got, ok, m2.Len())
		}
	})

	t.Run("FlushIsHarmless", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 3)
		for i := 10; i > 0; i-- {
			b.Set(i, i)
			b.Flush()
		}
		if got := b.SortedMap().Len(); got != 10 {
			t.Errorf("Len() = %d, want 10", got)
		}
	})

	t.Run("MapOwnsItsEntries", func(t *testing.T) {
		b := NewSortedBatchBuilder[int, int](nil, 0)
		for i := 0; i < 40; i++ {
			b.Set(i, i)
		}
		m := b.SortedMap()
		// Writing through a retained buffer must not reach the map. The
		// builder drops its buffer, so there is nothing to retain, and the
		// map's own leaves are copies.
		if b.entries != nil {
			t.Error("builder still holds its buffer after SortedMap()")
		}
		m2 := m.Set(0, -1)
		if got, _ := m.Get(0); got != 0 {
			t.Errorf("persistent Set changed the original: Get(0) = %d", got)
		}
		if got, _ := m2.Get(0); got != -1 {
			t.Errorf("Get(0) on the copy = %d, want -1", got)
		}
	})
}

// TestBatchSortedSetBuilder covers the set wrapper.
func TestBatchSortedSetBuilderLifecycle(t *testing.T) {
	b := NewBatchSortedSetBuilder[int](nil, 0)
	b.AddSlice([]int{5, 3, 5, 9, 1, 3})
	if b.Len() != 6 {
		t.Errorf("Len() = %d, want 6 (an upper bound)", b.Len())
	}
	b.Flush()
	set := b.SortedSet()
	if set == nil || set.Len() != 4 {
		t.Fatalf("set = %v", set)
	}
	var got []int
	for itr := set.Iterator(); !itr.Done(); {
		v, _ := itr.Next()
		got = append(got, v)
	}
	if !slices.Equal(got, []int{1, 3, 5, 9}) {
		t.Errorf("values %v, want [1 3 5 9]", got)
	}
	mustPanic(t, "Add", func() { b.Add(1) })
	mustPanic(t, "Len", func() { b.Len() })
	mustPanic(t, "second SortedSet()", func() { b.SortedSet() })

	empty := NewBatchSortedSetBuilder[string](nil, 0).SortedSet()
	if empty == nil || empty.Len() != 0 {
		t.Errorf("empty builder gave %v", empty)
	}
}
