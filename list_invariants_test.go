package immutable

import (
	"fmt"
	"reflect"
	"testing"
)

// checkListInvariants checks the representation rules from ADR-005, which the
// type system cannot: the tail is at most one leaf and its spare capacity holds
// no values, the trie is nil exactly when it holds nothing, the trie ends on a
// leaf boundary (so a full tail is one leaf), the trie's occupied slots are
// exactly the indexes origin through origin+trieLen-1, and every unoccupied leaf
// slot is zero, so no reference survives outside the list. Get and the iterator
// can't see most of these: a misplaced slot or a stale reference reads back fine
// until a later Append, Prepend or Slice trips over it.
func checkListInvariants[T any](t testing.TB, l *List[T]) {
	t.Helper()
	if len(l.tail) > listNodeSize {
		t.Errorf("I1: tail holds %d values, at most %d", len(l.tail), listNodeSize)
	}
	// Spare capacity behind the tail is not part of the list, so it must not hold
	// references (a builder's tail array is reused after each push to the trie).
	for i, v := range l.tail[len(l.tail):cap(l.tail)] {
		if !reflect.ValueOf(&v).Elem().IsZero() {
			t.Errorf("tail spare slot %d holds %v, want zero", len(l.tail)+i, v)
		}
	}
	trieLen := l.size - len(l.tail)
	if trieLen < 0 {
		t.Fatalf("size %d is smaller than the tail (%d)", l.size, len(l.tail))
	}
	if (l.root == nil) != (trieLen == 0) {
		t.Fatalf("I2: root nil = %v but the trie should hold %d values", l.root == nil, trieLen)
	}
	if l.root == nil {
		if l.origin != 0 {
			t.Errorf("I2: empty trie with origin %d, want 0", l.origin)
		}
		return
	}
	if (l.origin+trieLen)%listNodeSize != 0 {
		t.Errorf("I3: trie ends at index %d (origin %d + %d values), not on a leaf boundary",
			l.origin+trieLen, l.origin, trieLen)
	}
	if l.origin < 0 || l.origin+trieLen > l.cap() {
		t.Errorf("trie range [%d, %d) does not fit the root (capacity %d)", l.origin, l.origin+trieLen, l.cap())
	}
	var slots []int
	collectOccupied(t, l.root, 0, &slots)
	if len(slots) != trieLen {
		t.Errorf("trie has %d occupied slots, want %d", len(slots), trieLen)
		return
	}
	for i, idx := range slots {
		if idx != l.origin+i {
			t.Errorf("occupied slot %d is at trie index %d, want %d", i, idx, l.origin+i)
			return
		}
	}
}

// collectOccupied appends the trie index of every occupied leaf slot under n, in
// order, where base is the trie index of n's first slot. It fails the test if a
// slot that is not occupied holds a value.
func collectOccupied[T any](t testing.TB, n listNode[T], base int, out *[]int) {
	t.Helper()
	switch n := n.(type) {
	case *listLeafNode[T]:
		for i := range n.children {
			if n.occupied&(1<<i) != 0 {
				*out = append(*out, base+i)
			} else if !reflect.ValueOf(&n.children[i]).Elem().IsZero() {
				t.Errorf("slot %d (trie index %d) is not occupied but holds %v", i, base+i, n.children[i])
			}
		}
	case *listBranchNode[T]:
		span := 1 << (n.d * listNodeBits) // trie indexes covered by each child
		for i, c := range n.children {
			if c != nil {
				if c.depth() != n.d-1 {
					t.Errorf("child %d of a depth %d branch has depth %d", i, n.d, c.depth())
				}
				collectOccupied(t, c, base+i*span, out)
			}
		}
	default:
		t.Errorf("unexpected node type %T", n)
	}
}

// requireListIteration checks the iterator of l against want in both directions
// and from Seek at the places where the trie meets the tail.
func requireListIteration(t testing.TB, l *List[int], want []int) {
	t.Helper()
	n := len(want)
	itr := l.Iterator()
	for i := 0; i < n; i++ {
		if itr.Done() {
			t.Fatalf("forward: Done() before element %d of %d", i, n)
		}
		if idx, v := itr.Next(); idx != i || v != want[i] {
			t.Fatalf("forward: element %d returned (%d, %d), want (%d, %d)", i, idx, v, i, want[i])
		}
	}
	if !itr.Done() {
		t.Fatalf("forward: Done() is false after %d elements", n)
	}
	if idx, _ := itr.Next(); idx != -1 {
		t.Fatalf("forward: Next after the end returned index %d, want -1", idx)
	}
	if n == 0 {
		return
	}

	itr.Last()
	for i := n - 1; i >= 0; i-- {
		if itr.Done() {
			t.Fatalf("backward: Done() before element %d", i)
		}
		if idx, v := itr.Prev(); idx != i || v != want[i] {
			t.Fatalf("backward: element %d returned (%d, %d), want (%d, %d)", i, idx, v, i, want[i])
		}
	}
	if !itr.Done() {
		t.Fatalf("backward: Done() is false after reading element 0")
	}

	tl := l.trieLen()
	for _, at := range []int{0, 1, tl - 2, tl - 1, tl, tl + 1, n / 2, n - 2, n - 1} {
		if at < 0 || at >= n {
			continue
		}
		itr.Seek(at)
		for i := at; i < min(at+70, n); i++ {
			if idx, v := itr.Next(); idx != i || v != want[i] {
				t.Fatalf("Seek(%d) then Next: element %d returned (%d, %d), want (%d, %d)", at, i, idx, v, i, want[i])
			}
		}
		itr.Seek(at)
		for i := at; i >= max(at-70, 0); i-- {
			if idx, v := itr.Prev(); idx != i || v != want[i] {
				t.Fatalf("Seek(%d) then Prev: element %d returned (%d, %d), want (%d, %d)", at, i, idx, v, i, want[i])
			}
		}
	}
}

// TestListZeroValueIsEmpty checks what the type documents: the zero value of a
// List is an empty list, and works with every operation. Appending or
// prepending to it used to panic, because the root was assumed to be non-nil.
func TestListZeroValueIsEmpty(t *testing.T) {
	var z List[int]
	checkListInvariants(t, &z)
	requireListEquals(t, &z, nil)
	if z.Contains(1) || z.ContainsFunc(1, func(a, b int) bool { return a == b }) {
		t.Error("zero list contains a value")
	}
	requireListEquals(t, z.Slice(0, 0), nil)
	requireListEquals(t, z.Append(1).Append(2), []int{1, 2})
	requireListEquals(t, z.Prepend(1).Prepend(2), []int{2, 1})
	requireListEquals(t, &z, nil) // still empty

	for _, n := range []int{31, 32, 33, 100} {
		t.Run(fmt.Sprint("Append", n), func(t *testing.T) {
			var l List[int]
			p := &l
			for i := 0; i < n; i++ {
				p = p.Append(i)
			}
			requireListEquals(t, p, intsRange(n))
		})
	}
}

// TestListAppendAllocationContract pins what ADR-005 buys: an Append that does
// not fill the tail never touches the trie, whatever its size, so it allocates
// a new tail and a List and nothing else. A push of a full tail adds the leaf
// and the copied path (plus a new one-value tail), and Set inside the tail
// copies only the tail.
func TestListAppendAllocationContract(t *testing.T) {
	// Sizes with a trie of depth 0..3 and a tail that is neither empty nor full.
	for _, n := range []int{5, 40, 1030, 40001} {
		t.Run(fmt.Sprint("N", n), func(t *testing.T) {
			l := NewList(intsRange(n)...)
			if tl := len(l.tail); tl == 0 || tl == listNodeSize {
				t.Fatalf("setup: tail has %d values, want one that is neither empty nor full", tl)
			}
			if got := testing.AllocsPerRun(50, func() { sinkAny = l.Append(1) }); got != 2 {
				t.Errorf("Append into a non-full tail: %v allocs, want 2 (tail, List)", got)
			}
			if got := testing.AllocsPerRun(50, func() { sinkAny = l.Set(n-1, 1) }); got != 2 {
				t.Errorf("Set in the tail: %v allocs, want 2 (tail, List)", got)
			}
			if got := testing.AllocsPerRun(50, func() { sinkInt = l.Get(n - 1) }); got != 0 {
				t.Errorf("Get in the tail: %v allocs, want 0", got)
			}
		})
	}

	// A full tail is pushed as a leaf: the leaf, a copy of each branch on the path
	// from the root, a one-value tail and the List.
	for _, c := range []struct{ n, allocs int }{
		{128, 4},   // trie of 3 leaves under one branch: leaf, branch, tail, List
		{40000, 6}, // trie of depth 3: leaf, 3 branches, tail, List
	} {
		t.Run(fmt.Sprint("PushFullTail/N", c.n), func(t *testing.T) {
			l := NewList(intsRange(c.n)...)
			if len(l.tail) != listNodeSize {
				t.Fatalf("setup: tail has %d values, want %d", len(l.tail), listNodeSize)
			}
			if got := testing.AllocsPerRun(50, func() { sinkAny = l.Append(1) }); got != float64(c.allocs) {
				t.Errorf("Append onto a full tail: %v allocs, want %d", got, c.allocs)
			}
		})
	}

	t.Run("BuilderAppendIsAmortized", func(t *testing.T) {
		// The builder grows its tail in place and pushes it as a leaf when full,
		// so per value it allocates a fraction of an allocation.
		const n = 32 * 64
		got := testing.AllocsPerRun(5, func() {
			b := NewListBuilder[int]()
			for i := 0; i < n; i++ {
				b.Append(i)
			}
			sinkAny = b.List()
		})
		if perValue := got / n; perValue > 0.25 {
			t.Errorf("ListBuilder.Append: %.3f allocs per value, want <= 0.25", perValue)
		}
	})
}

// TestListContainsEveryPosition looks up each value of lists built in the ways
// that give different leaf layouts (a full first leaf, a partial first leaf from
// Prepend or Slice, a trie that ends in a tail) so a scan that misses the first
// or last slot of a leaf, or stops at a leaf boundary, is found.
func TestListContainsEveryPosition(t *testing.T) {
	equal := func(a, b int) bool { return a == b }
	build := map[string]func(n int) *List[int]{
		"New": func(n int) *List[int] { return NewList(intsRange(n)...) },
		"Append": func(n int) *List[int] {
			l := NewList[int]()
			for i := 0; i < n; i++ {
				l = l.Append(i)
			}
			return l
		},
		"Prepend": func(n int) *List[int] {
			l := NewList[int]()
			for i := n - 1; i >= 0; i-- {
				l = l.Prepend(i)
			}
			return l
		},
		"Slice": func(n int) *List[int] { // values 37..37+n-1, cut out of a larger list
			return NewList(intsRange(n+70)...).Slice(37, 37+n)
		},
	}
	for name, mk := range build {
		for _, n := range []int{1, 31, 32, 33, 64, 65, 100, 1025, 1100} {
			l := mk(n)
			offset := 0
			if name == "Slice" {
				offset = 37
			}
			for i := 0; i < n; i++ {
				v := i + offset
				if !l.Contains(v) {
					t.Fatalf("%s N=%d: Contains(%d) = false", name, n, v)
				}
				if !l.ContainsFunc(v, equal) {
					t.Fatalf("%s N=%d: ContainsFunc(%d) = false", name, n, v)
				}
			}
			if l.Contains(-1) || l.ContainsFunc(-1, equal) || l.Contains(n+offset+100) {
				t.Fatalf("%s N=%d: found a value that is not in the list", name, n)
			}
		}
	}
}

// TestListIteratorAllocations pins what 4.5 bought: an iterator is one small
// allocation (its path holds one entry per trie level, not a fixed 32), stepping
// allocates nothing, and Contains makes the iterator only when there is a trie.
func TestListIteratorAllocations(t *testing.T) {
	if size := reflect.TypeFor[ListIterator[int]]().Size(); size > 256 {
		t.Errorf("ListIterator[int] is %d bytes, want at most 256", size)
	}
	tailOnly := NewList(intsRange(20)...)
	big := NewList(intsRange(100000)...)
	if got := testing.AllocsPerRun(50, func() { sinkAny = big.Iterator() }); got != 1 {
		t.Errorf("Iterator(): %v allocs, want 1", got)
	}
	itr := big.Iterator()
	if got := testing.AllocsPerRun(1000, func() {
		if itr.Done() {
			itr.First()
		}
		_, sinkInt = itr.Next()
	}); got != 0 {
		t.Errorf("Next: %v allocs, want 0", got)
	}
	itr.Last()
	if got := testing.AllocsPerRun(1000, func() {
		if itr.Done() {
			itr.Last()
		}
		_, sinkInt = itr.Prev()
	}); got != 0 {
		t.Errorf("Prev: %v allocs, want 0", got)
	}
	if got := testing.AllocsPerRun(50, func() { sinkBool = tailOnly.Contains(-1) }); got != 0 {
		t.Errorf("Contains on a list with no trie: %v allocs, want 0", got)
	}
	if got := testing.AllocsPerRun(50, func() { sinkBool = big.Contains(-1) }); got != 1 {
		t.Errorf("Contains on a list with a trie: %v allocs, want 1 (the iterator)", got)
	}
}
