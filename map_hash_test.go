package immutable

import (
	"fmt"
	"strings"
	"testing"
)

// mapTrieStats describes the shape of a map's trie: how many values it holds,
// how many nodes it has, how deep the deepest value is, and how many collision
// nodes there are.
type mapTrieStats struct{ values, nodes, maxDepth, collisions int }

func trieStats[K, V any](m *Map[K, V]) mapTrieStats {
	var s mapTrieStats
	var walk func(n mapNode[K, V], depth int)
	walk = func(n mapNode[K, V], depth int) {
		switch n := n.(type) {
		case *mapValueNode[K, V]:
			s.values++
			s.maxDepth = max(s.maxDepth, depth)
		case *mapHashCollisionNode[K, V]:
			s.collisions++
			s.values += len(n.entries)
			s.maxDepth = max(s.maxDepth, depth)
		case *mapBitmapIndexedNode[K, V]:
			s.nodes++
			for _, c := range n.nodes {
				walk(c, depth+1)
			}
		case *mapHashArrayNode[K, V]:
			s.nodes++
			for _, c := range n.nodes {
				if c != nil {
					walk(c, depth+1)
				}
			}
		case *mapArrayNode[K, V]:
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
// average (bench-phase5-3-int-hash-2026-11-17.txt, in git history at 4334d13). A
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

// TestNewHasherIsChosenPerKeyType pins 5.4: NewHasher picks a hasher for the
// exact key type once, so Hash and Equal do not switch on the type of the key on
// every call. A named type still gets the reflection-based hasher.
func TestNewHasherIsChosenPerKeyType(t *testing.T) {
	check := func(name string, ok bool) {
		t.Helper()
		if !ok {
			t.Errorf("NewHasher(%s) did not return the specialized hasher", name)
		}
	}
	_, ok := NewHasher(int(0)).(integerHasher[int])
	check("int", ok)
	_, ok = NewHasher(int8(0)).(integerHasher[int8])
	check("int8", ok)
	_, ok = NewHasher(int16(0)).(integerHasher[int16])
	check("int16", ok)
	_, ok = NewHasher(int32(0)).(integerHasher[int32])
	check("int32", ok)
	_, ok = NewHasher(int64(0)).(integerHasher[int64])
	check("int64", ok)
	_, ok = NewHasher(uint(0)).(integerHasher[uint])
	check("uint", ok)
	_, ok = NewHasher(uint8(0)).(integerHasher[uint8])
	check("uint8", ok)
	_, ok = NewHasher(uint16(0)).(integerHasher[uint16])
	check("uint16", ok)
	_, ok = NewHasher(uint32(0)).(integerHasher[uint32])
	check("uint32", ok)
	_, ok = NewHasher(uint64(0)).(integerHasher[uint64])
	check("uint64", ok)
	_, ok = NewHasher(uintptr(0)).(integerHasher[uintptr])
	check("uintptr", ok)
	_, ok = NewHasher("").(stringHasher)
	check("string", ok)

	type myInt int
	type myString string
	if _, ok := NewHasher(myInt(0)).(*reflectHasher[myInt]); !ok {
		t.Error("NewHasher(myInt) should use the reflection-based hasher")
	}
	if _, ok := NewHasher(myString("")).(*reflectHasher[myString]); !ok {
		t.Error("NewHasher(myString) should use the reflection-based hasher")
	}
}

// checkIntegerHasher requires the hasher for K to hash a value as the 64-bit
// number it sign-extends to (hashUint64), and to compare by value.
func checkIntegerHasher[K integer](t *testing.T, vals ...K) {
	t.Helper()
	h := NewHasher(vals[0])
	for _, v := range vals {
		if got, want := h.Hash(v), hashUint64(uint64(v)); got != want {
			t.Errorf("%T: Hash(%v) = %#x, want %#x", v, v, got, want)
		}
		if !h.Equal(v, v) {
			t.Errorf("%T: Equal(%v, %v) = false", v, v, v)
		}
		for _, w := range vals {
			if (v == w) != h.Equal(v, w) {
				t.Errorf("%T: Equal(%v, %v) = %v", v, v, w, h.Equal(v, w))
			}
		}
	}
}

func TestIntegerHasherHashesLikeHashUint64(t *testing.T) {
	checkIntegerHasher(t, int(-1), 0, 1, 1<<31, 1<<40, -1<<40)
	checkIntegerHasher(t, int8(-1), 0, 1, 127, -128)
	checkIntegerHasher(t, int16(-1), 0, 1, 32767, -32768)
	checkIntegerHasher(t, int32(-1), 0, 1, 1<<31-1, -1<<31)
	checkIntegerHasher(t, int64(-1), 0, 1, 1<<62, -1<<63)
	checkIntegerHasher(t, uint(0), 1, 1<<32, ^uint(0))
	checkIntegerHasher(t, uint8(0), 1, 255)
	checkIntegerHasher(t, uint16(0), 1, 65535)
	checkIntegerHasher(t, uint32(0), 1, 1<<32-1)
	checkIntegerHasher(t, uint64(0), 1, 1<<32, ^uint64(0))
	checkIntegerHasher(t, uintptr(0), 1, ^uintptr(0))

	// A negative key is sign-extended: int8(-1) and int64(-1) hash alike, and
	// differently from uint8(255), which is the number 255.
	if a, b := NewHasher(int8(0)).Hash(-1), NewHasher(int64(0)).Hash(-1); a != b {
		t.Errorf("int8(-1) hashes to %#x but int64(-1) to %#x", a, b)
	}
	if a, b := NewHasher(int8(0)).Hash(-1), NewHasher(uint8(0)).Hash(255); a == b {
		t.Errorf("int8(-1) and uint8(255) both hash to %#x", a)
	}
}

// TestDefaultHashersDoNotAllocate: neither Hash nor Equal boxes its keys, even
// for values that would not fit the small-integer cache of an interface.
func TestDefaultHashersDoNotAllocate(t *testing.T) {
	ih := NewHasher(0)
	sh := NewHasher("")
	big, s := 1<<40, "a key that is long enough to matter"
	var sink uint32
	var eq bool
	for name, fn := range map[string]func(){
		"int Hash":     func() { sink += ih.Hash(big) },
		"int Equal":    func() { eq = ih.Equal(big, big+1) },
		"string Hash":  func() { sink += sh.Hash(s) },
		"string Equal": func() { eq = sh.Equal(s, s[:len(s)-1]) },
	} {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %v times, want 0", name, n)
		}
	}
	_, _ = sink, eq
}

func TestDefaultHasherEqualComparesContents(t *testing.T) {
	s := NewHasher("")
	if s.Equal("ab", "ac") || s.Equal("a", "") || !s.Equal("ab", "ab") {
		t.Fatal("string Equal must compare the whole content")
	}
	i := NewHasher(0)
	if i.Equal(1, 2) || i.Equal(2, 1) || !i.Equal(3, 3) {
		t.Fatal("int Equal must be ==")
	}
	m := NewMap[string, int](nil).Set("ab", 1).Set("ac", 2)
	if v, ok := m.Get("ab"); !ok || v != 1 {
		t.Fatalf("Get(ab) = %d, %v", v, ok)
	}
	if v, ok := m.Get("ac"); !ok || v != 2 {
		t.Fatalf("Get(ac) = %d, %v", v, ok)
	}
}

// diffComparer returns the difference of its arguments, so results other than
// -1, 0 and 1 are common. Comparer documents -1, 0 and 1, but a caller that
// returns any negative or positive number must not corrupt the tree.
type diffComparer struct{}

func (diffComparer) Compare(a, b int) int { return a - b }

func TestSortedMapToleratesComparerResultsBeyondOne(t *testing.T) {
	const n = 2000
	m := NewSortedMap[int, int](diffComparer{})
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	rng := splitmix(7)
	for i := n - 1; i > 0; i-- {
		j := rng.intn(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	for _, k := range order {
		m = m.Set(k*3, k)
	}
	if m.Len() != n {
		t.Fatalf("Len = %d, want %d", m.Len(), n)
	}
	for k := 0; k < n; k++ {
		if v, ok := m.Get(k * 3); !ok || v != k {
			t.Fatalf("Get(%d) = %d, %v", k*3, v, ok)
		}
		if _, ok := m.Get(k*3 + 1); ok {
			t.Fatalf("Get(%d) found a key that was never set", k*3+1)
		}
	}
	prev := -1
	for itr := m.Iterator(); !itr.Done(); {
		k, _, _ := itr.Next()
		if k <= prev {
			t.Fatalf("keys out of order: %d after %d", k, prev)
		}
		prev = k
	}
	for k := 0; k < n; k += 2 {
		m = m.Delete(k * 3)
	}
	if m.Len() != n/2 {
		t.Fatalf("Len after deletes = %d, want %d", m.Len(), n/2)
	}
	for k := 0; k < n; k++ {
		if _, ok := m.Get(k * 3); ok != (k%2 == 1) {
			t.Fatalf("Get(%d) found = %v after deleting even k", k*3, ok)
		}
	}
}

func TestNewHasherInterfaceKeys(t *testing.T) {
	m := NewMap[any, int](nil)
	m = m.Set("a", 1).Set(2, 2).Set("b", 3)
	for k, want := range map[any]int{"a": 1, 2: 2, "b": 3} {
		if v, ok := m.Get(k); !ok || v != want {
			t.Fatalf("Get(%v) = %v, %v; want %v", k, v, ok, want)
		}
	}
	s := NewSet[any](nil, "x", 7)
	if !s.Has("x") || !s.Has(7) || s.Has("y") {
		t.Fatal("interface-keyed set lost keys")
	}
}

func TestInterfaceKeysKeepDynamicTypes(t *testing.T) {
	m := NewMap[any, string](nil).Set(int(5), "int").Set(int64(5), "int64").Set(uint8(5), "uint8").Set("5", "string")
	if m.Len() != 4 {
		t.Fatalf("Len = %d, want 4 (int(5), int64(5), uint8(5) and \"5\" are different keys)", m.Len())
	}
	for k, want := range map[any]string{int(5): "int", int64(5): "int64", uint8(5): "uint8", "5": "string"} {
		if v, _ := m.Get(k); v != want {
			t.Fatalf("Get(%T %v) = %q, want %q", k, k, v, want)
		}
	}
	m = m.Delete(int64(5))
	if m.Len() != 3 {
		t.Fatalf("Len after Delete = %d, want 3", m.Len())
	}
}

func TestContainsFuncNilEqualPanicsOnEmptyList(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	NewList[int]().ContainsFunc(1, nil)
}

func TestZeroValueBuilderMessageMentionsZeroValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "zero value") {
			t.Fatalf("panic = %v, want a message mentioning the zero value", r)
		}
	}()
	var b ListBuilder[int]
	b.Append(1)
}
