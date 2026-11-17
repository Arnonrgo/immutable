package immutable

import (
	"strings"
	"testing"
)

// panicMessage runs f and returns the string it panicked with. It fails the
// test if f does not panic, and returns "" in that case.
func panicMessage(t *testing.T, what string, f func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected a panic", what)
			return
		}
		s, ok := r.(string)
		if !ok {
			t.Errorf("%s: panic value %v (%T) is not a string", what, r, r)
			return
		}
		msg = s
	}()
	f()
	return ""
}

// requireMessage fails unless f panics with a message that has every want
// fragment and none of the avoid fragments.
func requireMessage(t *testing.T, what string, f func(), want []string, avoid ...string) {
	t.Helper()
	msg := panicMessage(t, what, f)
	if msg == "" {
		return
	}
	for _, w := range want {
		if !strings.Contains(msg, w) {
			t.Errorf("%s: panic %q does not mention %q", what, msg, w)
		}
	}
	for _, a := range avoid {
		if strings.Contains(msg, a) {
			t.Errorf("%s: panic %q wrongly mentions %q", what, msg, a)
		}
	}
}

// builderOps are the operations every set builder supports, so the two set
// builders share one lifecycle test.
type builderOps struct {
	add    func(int)
	del    func(int)
	has    func(int) bool
	length func() int
	finish func() int // the terminal call; returns the built set's Len
}

func setBuilderOps(b *SetBuilder[int]) builderOps {
	return builderOps{
		add:    b.Add,
		del:    b.Delete,
		has:    b.Has,
		length: b.Len,
		finish: func() int { s := b.Set(); return s.Len() },
	}
}

func sortedSetBuilderOps(b *SortedSetBuilder[int]) builderOps {
	return builderOps{
		add:    b.Add,
		del:    b.Delete,
		has:    b.Has,
		length: b.Len,
		finish: func() int { s := b.SortedSet(); return s.Len() },
	}
}

func TestSetBuilderLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		terminal  string // name of the terminal method
		duplicate string
		fresh     func() builderOps
		zero      func() builderOps
		ctor      string
	}{
		{
			name: "SetBuilder", terminal: "Set()", duplicate: "duplicate call to fetch set",
			fresh: func() builderOps { return setBuilderOps(NewSetBuilder[int](nil)) },
			zero:  func() builderOps { return setBuilderOps(&SetBuilder[int]{}) },
			ctor:  "NewSetBuilder",
		},
		{
			name: "SortedSetBuilder", terminal: "SortedSet()", duplicate: "duplicate call to fetch sorted set",
			fresh: func() builderOps { return sortedSetBuilderOps(NewSortedSetBuilder[int](nil)) },
			zero:  func() builderOps { return sortedSetBuilderOps(&SortedSetBuilder[int]{}) },
			ctor:  "NewSortedSetBuilder",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("UsableUntilTerminalCall", func(t *testing.T) {
				b := tc.fresh()
				if b.length() != 0 || b.has(1) {
					t.Fatalf("new builder is not empty")
				}
				b.add(1)
				b.add(2)
				b.add(2)
				b.del(1)
				if b.length() != 1 || b.has(1) || !b.has(2) {
					t.Fatalf("Len=%d Has(1)=%v Has(2)=%v, want 1 false true", b.length(), b.has(1), b.has(2))
				}
				if got := b.finish(); got != 1 {
					t.Fatalf("built set has %d elements, want 1", got)
				}
			})

			t.Run("EveryMethodPanicsAfterTerminalCall", func(t *testing.T) {
				b := tc.fresh()
				b.add(1)
				b.finish()
				invalid := "builder invalid after " + tc.terminal
				requireMessage(t, "Add", func() { b.add(2) }, []string{invalid})
				requireMessage(t, "Delete", func() { b.del(1) }, []string{invalid})
				requireMessage(t, "Has", func() { b.has(1) }, []string{invalid})
				requireMessage(t, "Len", func() { b.length() }, []string{invalid})
				requireMessage(t, "second terminal call", func() { b.finish() }, []string{tc.duplicate})
				// A refused call must leave the builder invalid.
				requireMessage(t, "Add again", func() { b.add(3) }, []string{invalid})
			})

			t.Run("TerminalCallOnEmptyBuilderStillInvalidates", func(t *testing.T) {
				b := tc.fresh()
				if got := b.finish(); got != 0 {
					t.Fatalf("empty builder built a set with %d elements", got)
				}
				requireMessage(t, "Add", func() { b.add(1) }, []string{"builder invalid after " + tc.terminal})
				requireMessage(t, "second terminal call", func() { b.finish() }, []string{tc.duplicate})
			})

			// The zero value was never valid, which is not the same as being
			// used up: the message must point at the constructor, not claim that
			// the terminal method was called.
			t.Run("ZeroValuePanicsAndSaysWhy", func(t *testing.T) {
				b := tc.zero()
				want := []string{"zero value", tc.ctor}
				avoid := []string{"after " + tc.terminal, "duplicate"}
				requireMessage(t, "Add", func() { b.add(1) }, want, avoid...)
				requireMessage(t, "Delete", func() { b.del(1) }, want, avoid...)
				requireMessage(t, "Has", func() { b.has(1) }, want, avoid...)
				requireMessage(t, "Len", func() { b.length() }, want, avoid...)
				requireMessage(t, "terminal call", func() { b.finish() }, want, avoid...)
			})
		})
	}
}

func TestSetBuilderLifecycleKeepsBuiltSet(t *testing.T) {
	b := NewSetBuilder[int](nil)
	b.Add(1)
	b.Add(2)
	s := b.Set()
	requireMessage(t, "Add", func() { b.Add(3) }, []string{"after Set()"})
	if s.Len() != 2 || !s.Has(1) || !s.Has(2) || s.Has(3) {
		t.Fatalf("built set changed after the builder was refused: Len=%d", s.Len())
	}

	sb := NewSortedSetBuilder[int](nil)
	sb.Add(2)
	sb.Add(1)
	ss := sb.SortedSet()
	requireMessage(t, "Add", func() { sb.Add(3) }, []string{"after SortedSet()"})
	if got := ss.Items(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("built sorted set changed after the builder was refused: %v", got)
	}
}

func TestQueueBuilderLifecycle(t *testing.T) {
	drain := func(t *testing.T, q *Queue[int]) []int {
		t.Helper()
		var out []int
		for {
			var v int
			var ok bool
			q, v, ok = q.Dequeue()
			if !ok {
				return out
			}
			out = append(out, v)
		}
	}
	equal := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	t.Run("EveryMethodPanicsAfterQueue", func(t *testing.T) {
		b := NewQueueBuilder[int]()
		b.Enqueue(1)
		q := b.Queue()
		invalid := "builder invalid after Queue()"
		requireMessage(t, "Enqueue", func() { b.Enqueue(2) }, []string{invalid})
		requireMessage(t, "EnqueueSlice", func() { b.EnqueueSlice([]int{2}) }, []string{invalid})
		requireMessage(t, "Len", func() { b.Len() }, []string{invalid})
		requireMessage(t, "second Queue", func() { b.Queue() }, []string{"duplicate call to fetch queue"})
		requireMessage(t, "Enqueue again", func() { b.Enqueue(3) }, []string{invalid})
		if got := drain(t, q); !equal(got, []int{1}) {
			t.Fatalf("built queue changed after the builder was refused: %v", got)
		}
	})

	t.Run("QueueOnEmptyBuilderStillInvalidates", func(t *testing.T) {
		b := NewQueueBuilder[int]()
		if q := b.Queue(); q == nil || !q.Empty() {
			t.Fatalf("empty builder built %v, want an empty non-nil queue", q)
		}
		requireMessage(t, "Enqueue", func() { b.Enqueue(1) }, []string{"builder invalid after Queue()"})
		requireMessage(t, "second Queue", func() { b.Queue() }, []string{"duplicate call to fetch queue"})
	})

	// A nil *Queue is the empty queue everywhere in the package, so the zero
	// builder is an empty builder rather than an error.
	t.Run("ZeroValueIsAnEmptyBuilder", func(t *testing.T) {
		var b QueueBuilder[int]
		if b.Len() != 0 {
			t.Fatalf("zero builder Len=%d, want 0", b.Len())
		}
		b.Enqueue(1)
		b.EnqueueSlice([]int{2, 3})
		if b.Len() != 3 {
			t.Fatalf("Len=%d, want 3", b.Len())
		}
		if got := drain(t, b.Queue()); !equal(got, []int{1, 2, 3}) {
			t.Fatalf("zero builder built %v, want [1 2 3]", got)
		}
		requireMessage(t, "Enqueue", func() { b.Enqueue(4) }, []string{"builder invalid after Queue()"})
	})

	t.Run("ZeroValueQueueIsNonNilAndInvalidates", func(t *testing.T) {
		var b QueueBuilder[int]
		q := b.Queue()
		if q == nil || !q.Empty() {
			t.Fatalf("unused zero builder built %v, want an empty non-nil queue", q)
		}
		requireMessage(t, "Enqueue", func() { b.Enqueue(1) }, []string{"builder invalid after Queue()"})
		requireMessage(t, "Len", func() { b.Len() }, []string{"builder invalid after Queue()"})
		requireMessage(t, "second Queue", func() { b.Queue() }, []string{"duplicate call to fetch queue"})
	})
}
