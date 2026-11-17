package immutable

import (
	"fmt"
	"slices"
	"testing"
)

// listTrieDepth returns the depth of the root of the list's trie; a list with
// no trie (nothing but a tail) counts as depth 0, the same as a single leaf.
func listTrieDepth[T any](l *List[T]) uint {
	if l.root == nil {
		return 0
	}
	return l.root.depth()
}

// minimalTrieDepth returns the smallest depth whose root can hold n values
// (a leaf holds 32, each level above multiplies that by 32).
func minimalTrieDepth(n int) uint {
	d, capacity := uint(0), listNodeSize
	for capacity < n {
		d++
		capacity *= listNodeSize
	}
	return d
}

// bulkBoundarySizes are sizes around each node-capacity boundary.
var bulkBoundarySizes = []int{
	0, 1, 31, 32, 33, 34, 63, 64, 65, 1023, 1024, 1025, 1056, 2047, 2048, 32767, 32768, 32769, 40000,
}

// requireTrieShape checks the structure of a list built in bulk: up to one
// leaf of values it is only a tail; otherwise the last leaf (1 to 32 values) is
// the tail and the rest is a trie of minimum depth with origin 0 whose occupied
// slots are exactly the values in it. Get and the iterator cannot see a wrong
// occupancy mask, but Slice relies on it to drop references to elements outside
// the range.
func requireTrieShape[T any](t *testing.T, l *List[T]) {
	t.Helper()
	n := l.Len()
	checkListInvariants(t, l)
	if n <= listNodeSize {
		if l.root != nil || len(l.tail) != n {
			t.Fatalf("n=%d: want a tail only, got root %T and a tail of %d", n, l.root, len(l.tail))
		}
		return
	}
	if want := (n-1)%listNodeSize + 1; len(l.tail) != want {
		t.Errorf("n=%d: tail of %d values, want %d", n, len(l.tail), want)
	}
	trieLen := l.trieLen()
	if got, want := listTrieDepth(l), minimalTrieDepth(trieLen); got != want {
		t.Errorf("n=%d: root depth %d, want %d for %d values in the trie", n, got, want, trieLen)
	}
	if l.origin != 0 {
		t.Errorf("n=%d: origin %d, want 0", n, l.origin)
	}
	if got := occupiedCount(l.root); got != trieLen {
		t.Errorf("n=%d: %d occupied slots, want %d", n, got, trieLen)
	}
}

func intsUpTo(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i*7 + 1
	}
	return out
}

// TestNewList_BulkBuild checks the contents and shape of NewList at each size.
func TestNewList_BulkBuild(t *testing.T) {
	for _, n := range bulkBoundarySizes {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			values := intsUpTo(n)
			input := slices.Clone(values)
			l := NewList(input...)
			requireListEquals(t, l, values)

			// NewList must copy: changing the input afterwards is invisible.
			for i := range input {
				input[i] = -1
			}
			requireListEquals(t, l, values)

			requireTrieShape(t, l)
		})
	}
}

// TestNewList_BulkBuild_Mutations applies every persistent operation to a
// bulk-built list and compares against the same operation on a slice. This
// guards the shape assumptions (partly filled last nodes, nil children,
// occupancy masks, minimum depth) that Append, Prepend, Set and Slice make.
func TestNewList_BulkBuild_Mutations(t *testing.T) {
	for _, n := range bulkBoundarySizes {
		if n == 0 {
			continue
		}
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			values := intsUpTo(n)
			base := NewList(values...)

			t.Run("Append", func(t *testing.T) {
				l := base
				want := slices.Clone(values)
				for i := 0; i < 70; i++ { // crosses at least one leaf boundary
					l = l.Append(-i)
					want = append(want, -i)
				}
				requireListEquals(t, l, want)
				requireListEquals(t, base, values) // original unchanged
			})

			t.Run("Prepend", func(t *testing.T) {
				l := base
				want := slices.Clone(values)
				for i := 0; i < 70; i++ {
					l = l.Prepend(-i)
					want = append([]int{-i}, want...)
				}
				requireListEquals(t, l, want)
				requireListEquals(t, base, values)
			})

			t.Run("Set", func(t *testing.T) {
				for _, idx := range []int{0, n / 2, n - 1} {
					want := slices.Clone(values)
					want[idx] = -99
					requireListEquals(t, base.Set(idx, -99), want)
				}
				requireListEquals(t, base, values)
			})

			t.Run("Slice", func(t *testing.T) {
				cuts := [][2]int{{0, n}, {0, n / 2}, {n / 2, n}, {n / 3, 2 * n / 3}, {n - 1, n}, {0, 1}, {n / 2, n / 2}}
				for _, c := range cuts {
					requireListEquals(t, base.Slice(c[0], c[1]), values[c[0]:c[1]])
				}
				requireListEquals(t, base, values)
			})

			t.Run("Contains", func(t *testing.T) {
				if !base.Contains(values[n-1]) || !base.Contains(values[0]) {
					t.Error("expected first and last values to be found")
				}
				if base.Contains(-1) {
					t.Error("unexpected value found")
				}
			})
		})
	}
}

// TestNewList_StringsAndPointers uses element types with pointers so the leaf
// copy is exercised with values that the garbage collector must trace.
func TestNewList_StringsAndPointers(t *testing.T) {
	const n = 5000
	strs := make([]string, n)
	ptrs := make([]*int, n)
	for i := range strs {
		strs[i] = fmt.Sprintf("s%d", i)
		v := i
		ptrs[i] = &v
	}
	ls := NewList(strs...)
	lp := NewList(ptrs...)
	for i := 0; i < n; i++ {
		if ls.Get(i) != strs[i] {
			t.Fatalf("string mismatch at %d", i)
		}
		if lp.Get(i) != ptrs[i] {
			t.Fatalf("pointer mismatch at %d", i)
		}
	}
}

// occupiedCount walks a trie and counts the slots marked occupied. For a
// well-formed list it equals the number of values in the trie (Len() less the
// tail): Slice is documented to drop references to elements outside the range,
// which only happens if the occupancy masks and nil children are right. Get and
// the iterator cannot see a wrong mask.
func occupiedCount[T any](n listNode[T]) int {
	switch n := n.(type) {
	case *listLeafNode[T]:
		return bitsOnes(n.occupied)
	case *listBranchNode[T]:
		total := 0
		for _, c := range n.children {
			if c != nil {
				total += occupiedCount(c)
			}
		}
		return total
	case nil:
		return 0
	default:
		return -1 // not a trie node
	}
}

func bitsOnes(x uint32) int {
	c := 0
	for ; x != 0; x &= x - 1 {
		c++
	}
	return c
}

// TestNewList_BulkBuild_Occupancy checks the occupancy masks of bulk-built
// tries directly, before and after Slice, and compares with an append-built
// list so a regression in either construction path is visible.
func TestNewList_BulkBuild_Occupancy(t *testing.T) {
	for _, n := range bulkBoundarySizes {
		if n <= listNodeSize {
			continue
		}
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			values := intsUpTo(n)
			bulk := NewList(values...)
			built := NewList[int]()
			for _, v := range values {
				built = built.Append(v)
			}
			for name, l := range map[string]*List[int]{"bulk": bulk, "appended": built} {
				if got, want := occupiedCount(l.root), l.trieLen(); got != want {
					t.Errorf("%s: %d occupied slots, want %d", name, got, want)
				}
				for _, c := range [][2]int{{1, n}, {0, n - 1}, {n / 3, 2 * n / 3}, {n / 2, n/2 + 1}} {
					s := l.Slice(c[0], c[1])
					checkListInvariants(t, s)
					if got, want := occupiedCount(s.root)+len(s.tail), c[1]-c[0]; got != want {
						t.Errorf("%s: Slice(%d,%d) holds %d values, want %d", name, c[0], c[1], got, want)
					}
				}
			}
		})
	}
}
