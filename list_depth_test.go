package immutable

import (
	"fmt"
	"slices"
	"testing"
)

// depthCheckSizes are the sizes at which lists made by Append or Prepend are
// checked: around every capacity boundary of the trie (a leaf holds 32 values,
// and each level above multiplies that by 32).
var depthCheckSizes = []int{
	33, 34, 35, 63, 64, 65, 1023, 1024, 1025, 1026, 2048, 32767, 32768, 32769, 32770,
}

func requireMinimalDepth(t *testing.T, how string, l *List[int]) {
	t.Helper()
	if got, want := listTrieDepth(l), minimalTrieDepth(l.Len()); got != want {
		t.Errorf("%s: n=%d has root depth %d, want %d (the smallest that holds %d values)",
			how, l.Len(), got, want, l.Len())
	}
}

// TestListAppendAndPrependKeepMinimalDepth checks that a list grown one value at
// a time is no deeper than its size needs. An extra level costs every Get and Set
// one more step and every Set one more node copy. The check is made at each
// capacity boundary, and the contents are compared at the same points.
func TestListAppendAndPrependKeepMinimalDepth(t *testing.T) {
	last := depthCheckSizes[len(depthCheckSizes)-1]
	atCheck := make(map[int]bool, len(depthCheckSizes))
	for _, n := range depthCheckSizes {
		atCheck[n] = true
	}

	t.Run("Append", func(t *testing.T) {
		l := NewList[int]()
		want := make([]int, 0, last)
		for i := 0; i < last; i++ {
			l = l.Append(i)
			want = append(want, i)
			if atCheck[l.Len()] {
				requireMinimalDepth(t, "Append", l)
				if l.origin != 0 {
					t.Errorf("Append: n=%d has origin %d, want 0", l.Len(), l.origin)
				}
				requireListEquals(t, l, want)
			}
		}
	})

	t.Run("Prepend", func(t *testing.T) {
		l := NewList[int]()
		var want []int
		for i := 0; i < last; i++ {
			l = l.Prepend(i)
			want = append(want, 0)
			copy(want[1:], want)
			want[0] = i
			if atCheck[l.Len()] {
				requireMinimalDepth(t, "Prepend", l)
				requireListEquals(t, l, want)
			}
		}
	})

	t.Run("ListBuilderAppend", func(t *testing.T) {
		b := NewListBuilder[int]()
		for i := 0; i < last; i++ {
			b.Append(i)
			if atCheck[b.Len()] {
				requireMinimalDepth(t, "ListBuilder.Append", b.list)
			}
		}
		l := b.List()
		requireListEquals(t, l, intsRange(last))
	})

	// A list made in bulk starts at the smallest depth; Append must not add a
	// level until its last slot is used.
	t.Run("AppendToBulkBuiltList", func(t *testing.T) {
		for _, n := range []int{32, 33, 1023, 1024, 1025, 32767, 32768} {
			t.Run(fmt.Sprint("N", n), func(t *testing.T) {
				l := NewList(intsRange(n)...)
				want := intsRange(n)
				for i := 0; i < 70; i++ {
					l = l.Append(n + i)
					want = append(want, n+i)
					requireMinimalDepth(t, "Append after NewList", l)
				}
				requireListEquals(t, l, want)
			})
		}
	})
}

func intsRange(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// TestListMatchesModel applies random Append, Prepend, Set and Slice operations
// to a pool of list versions and to a slice model of each, one operation at a
// time, and checks the result and the version the operation started from. It
// mixes growing at both ends and slicing, the cases where where an index falls
// relative to the root depends on origin and depth being right.
func TestListMatchesModel(t *testing.T) {
	type version struct {
		l    *List[int]
		want []int
	}
	rng := splitmix(7)

	var pool []version
	for _, n := range []int{0, 1, 31, 32, 33, 1023, 1024, 1025, 2049} {
		vals := intsRange(n)
		pool = append(pool, version{NewList(vals...), slices.Clone(vals)})
	}
	// A list that was grown by Append, and one that was grown by Prepend, so the
	// pool holds trees with and without an origin offset.
	grown := NewList[int]()
	var grownWant []int
	for i := 0; i < 1100; i++ {
		grown = grown.Append(i)
		grownWant = append(grownWant, i)
	}
	pool = append(pool, version{grown, grownWant})
	pre := NewList[int]()
	var preWant []int
	for i := 0; i < 1100; i++ {
		pre = pre.Prepend(i)
		preWant = append([]int{i}, preWant...)
	}
	pool = append(pool, version{pre, preWant})

	const maxPool = 16
	for step := 0; step < 1500; step++ {
		idx := rng.intn(len(pool))
		cur := pool[idx]
		n := len(cur.want)
		var next version
		switch rng.intn(6) {
		case 0, 1:
			next = version{cur.l.Append(-step), append(slices.Clone(cur.want), -step)}
		case 2, 3:
			next = version{cur.l.Prepend(-step), append([]int{-step}, cur.want...)}
		case 4:
			if n == 0 {
				continue
			}
			i := rng.intn(n)
			w := slices.Clone(cur.want)
			w[i] = -step
			next = version{cur.l.Set(i, -step), w}
		default:
			start := rng.intn(n + 1)
			end := start + rng.intn(n-start+1)
			next = version{cur.l.Slice(start, end), slices.Clone(cur.want[start:end])}
		}
		requireListEquals(t, next.l, next.want)
		requireListEquals(t, cur.l, cur.want) // the version it came from is untouched
		if len(pool) < maxPool {
			pool = append(pool, next)
		} else {
			pool[rng.intn(len(pool))] = next
		}
	}
}

// TestListBuilderMatchesModel runs the same kind of random operations through
// ListBuilder, which changes its nodes in place instead of copying them.
func TestListBuilderMatchesModel(t *testing.T) {
	for _, start := range []int{0, 31, 32, 1023, 1024, 1025} {
		t.Run(fmt.Sprint("Start", start), func(t *testing.T) {
			rng := splitmix(uint64(100 + start))
			want := intsRange(start)
			b := NewListBuilder[int]()
			for _, v := range want {
				b.Append(v)
			}
			for step := 0; step < 1200; step++ {
				n := len(want)
				switch rng.intn(5) {
				case 0, 1:
					b.Append(-step)
					want = append(want, -step)
				case 2:
					b.Prepend(-step)
					want = append([]int{-step}, want...)
				case 3:
					if n > 0 {
						i := rng.intn(n)
						b.Set(i, -step)
						want[i] = -step
					}
				default:
					if n > 0 && rng.intn(4) == 0 { // slice rarely, so the list keeps its size
						s := rng.intn(n/4 + 1)
						e := n - rng.intn(n/4+1)
						b.Slice(s, e)
						want = slices.Clone(want[s:e])
					}
				}
				if step%50 == 0 {
					requireListEquals(t, b.list, want)
				}
			}
			requireListEquals(t, b.List(), want)
		})
	}
}
