package immutable

import (
	"slices"
	"sync"
	"testing"
)

// splitmix is a small deterministic generator (splitmix64), so the model test
// picks the same operations on every run without a dependency on math/rand.
type splitmix uint64

func (s *splitmix) intn(n int) int {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return int((z ^ (z >> 31)) % uint64(n))
}

// queueContents reads the queue through its iterator and checks the iterator's
// own contract on the way: indexes count up from 0, Done turns true exactly
// when the last element has been returned, and Next after that reports !ok.
func queueContents(t *testing.T, q *Queue[int]) []int {
	t.Helper()
	var out []int
	itr := q.Iterator()
	for !itr.Done() {
		idx, v, ok := itr.Next()
		if !ok {
			t.Fatalf("Next reported !ok after %d elements while Done() was false", len(out))
		}
		if idx != len(out) {
			t.Fatalf("Next returned index %d, want %d", idx, len(out))
		}
		out = append(out, v)
	}
	if idx, _, ok := itr.Next(); ok || idx != -1 {
		t.Fatalf("Next after Done returned index=%d ok=%v, want -1 false", idx, ok)
	}
	return out
}

// requireQueue checks every read operation of q against the expected contents.
func requireQueue(t *testing.T, q *Queue[int], want []int) {
	t.Helper()
	if q.Len() != len(want) {
		t.Fatalf("Len() = %d, want %d", q.Len(), len(want))
	}
	if q.Empty() != (len(want) == 0) {
		t.Fatalf("Empty() = %v with %d elements", q.Empty(), len(want))
	}
	v, ok := q.Peek()
	if len(want) == 0 {
		if ok {
			t.Fatalf("Peek() on an empty queue returned %d, true", v)
		}
	} else if !ok || v != want[0] {
		t.Fatalf("Peek() = %d, %v, want %d, true", v, ok, want[0])
	}
	if got := queueContents(t, q); !slices.Equal(got, want) {
		t.Fatalf("iterator gave %v, want %v", got, want)
	}
}

// TestQueueMatchesModel runs random operations against a pool of queue
// versions and a slice model of each. Operations pick any existing version, not
// just the newest, so the same version is read and extended many times, which is
// what persistent use means.
func TestQueueMatchesModel(t *testing.T) {
	type version struct {
		q    *Queue[int]
		want []int
	}
	rng := splitmix(1)

	// Start from queues of different shapes: empty, front only (NewQueue and
	// NewQueueOf), and built by the builder.
	pool := []version{{NewQueue[int](), nil}}
	for _, n := range []int{1, 2, 5, 33} {
		vals := make([]int, n)
		for i := range vals {
			vals[i] = -1 - i
		}
		pool = append(pool, version{NewQueueOf(vals), slices.Clone(vals)})
		pool = append(pool, version{NewQueue(vals...), slices.Clone(vals)})
		b := NewQueueBuilder[int]()
		b.EnqueueSlice(vals)
		pool = append(pool, version{b.Queue(), slices.Clone(vals)})
	}

	const maxPool = 24
	for step := 0; step < 6000; step++ {
		cur := pool[rng.intn(len(pool))]
		var next version
		switch rng.intn(8) {
		case 0, 1, 2:
			next = version{cur.q.Enqueue(step), append(slices.Clone(cur.want), step)}
		case 3, 4, 5:
			q, v, ok := cur.q.Dequeue()
			if len(cur.want) == 0 {
				if ok || q != nil {
					t.Fatalf("step %d: Dequeue on an empty queue returned %v, %d, %v", step, q, v, ok)
				}
				continue
			}
			if !ok || v != cur.want[0] {
				t.Fatalf("step %d: Dequeue = %d, %v, want %d, true", step, v, ok, cur.want[0])
			}
			next = version{q, slices.Clone(cur.want[1:])}
		case 6:
			// Read the same version again: reading must not change it.
			requireQueue(t, cur.q, cur.want)
			continue
		default:
			vals := []int{step, step + 1, step + 2}
			b := NewQueueBuilder[int]()
			b.EnqueueSlice(vals)
			q := b.Queue()
			next = version{q, vals}
			// Extend an existing version with the same values, one at a time.
			ext := cur.q
			for _, v := range vals {
				ext = ext.Enqueue(v)
			}
			requireQueue(t, ext, append(slices.Clone(cur.want), vals...))
		}
		requireQueue(t, next.q, next.want)
		requireQueue(t, cur.q, cur.want) // the version it came from is untouched
		if len(next.want) > 400 {
			continue // keep the pool small enough to check every step
		}
		if len(pool) < maxPool {
			pool = append(pool, next)
		} else {
			pool[rng.intn(len(pool))] = next
		}
	}
}

// TestQueueRepeatedReadsAreStable reads one back-only queue (the shape the
// Enqueue loop makes) over and over, and branches it, and checks nothing moves.
func TestQueueRepeatedReadsAreStable(t *testing.T) {
	const n = 100
	want := make([]int, n)
	for i := range want {
		want[i] = i
	}
	q := queueFromEnqueue(n)

	for round := 0; round < 3; round++ {
		requireQueue(t, q, want)
		next, v, ok := q.Dequeue()
		if !ok || v != 0 {
			t.Fatalf("round %d: Dequeue = %d, %v, want 0, true", round, v, ok)
		}
		requireQueue(t, next, want[1:])
	}

	// Two branches from the same version do not see each other.
	a, _, _ := q.Dequeue()
	b, _, _ := q.Dequeue()
	a = a.Enqueue(1000)
	requireQueue(t, b, want[1:])
	requireQueue(t, a, append(slices.Clone(want[1:]), 1000))
	requireQueue(t, q, want)
}

// TestQueueConcurrentReads has several goroutines read a freshly built
// back-only queue at the same moment. It is the case where the first reader
// prepares the queue for the others, so it is the one the race detector should
// watch.
func TestQueueConcurrentReads(t *testing.T) {
	const n, readers, rounds = 50, 8, 200
	want := make([]int, n)
	for i := range want {
		want[i] = i
	}
	for round := 0; round < rounds; round++ {
		q := queueFromEnqueue(n)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for r := 0; r < readers; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				switch r % 3 {
				case 0:
					if v, ok := q.Peek(); !ok || v != 0 {
						t.Errorf("Peek = %d, %v, want 0, true", v, ok)
					}
				case 1:
					next, v, ok := q.Dequeue()
					if !ok || v != 0 || next.Len() != n-1 {
						t.Errorf("Dequeue = len %d, %d, %v, want len %d, 0, true", next.Len(), v, ok, n-1)
					}
				default:
					itr := q.Iterator()
					for i := 0; i < n; i++ {
						if _, v, ok := itr.Next(); !ok || v != i {
							t.Errorf("Next at %d = %d, %v", i, v, ok)
							return
						}
					}
					if !itr.Done() {
						t.Errorf("iterator not done after %d elements", n)
					}
				}
			}()
		}
		close(start)
		wg.Wait()
		if t.Failed() {
			return
		}
		requireQueue(t, q, want)
	}
}

// TestQueueIteratorShapes checks the iterator on every shape of queue: empty,
// front only, back only, and both.
func TestQueueIteratorShapes(t *testing.T) {
	mixed := NewQueueOf([]int{0, 1, 2})
	for i := 3; i < 7; i++ {
		mixed = mixed.Enqueue(i)
	}
	drained := queueFromEnqueue(6) // back only; drop two so the back list is partly used
	drained, _, _ = drained.Dequeue()
	drained, _, _ = drained.Dequeue()
	drained = drained.Enqueue(6)

	for _, tc := range []struct {
		name string
		q    *Queue[int]
		want []int
	}{
		{"Nil", nil, nil},
		{"Empty", NewQueue[int](), nil},
		{"FrontOnly", NewQueueOf([]int{0, 1, 2, 3}), []int{0, 1, 2, 3}},
		{"BackOnly", queueFromEnqueue(4), []int{0, 1, 2, 3}},
		{"FrontAndBack", mixed, []int{0, 1, 2, 3, 4, 5, 6}},
		{"AfterDequeues", drained, []int{2, 3, 4, 5, 6}},
		{"SingleBack", queueFromEnqueue(1), []int{0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireQueue(t, tc.q, tc.want)

			itr := tc.q.Iterator()
			itr.First() // positions at the first element, which is where it already is
			if itr.Done() != (len(tc.want) == 0) {
				t.Fatalf("Done() = %v before reading, with %d elements", itr.Done(), len(tc.want))
			}
			for i, w := range tc.want {
				if itr.Done() {
					t.Fatalf("Done() is true before element %d of %d", i, len(tc.want))
				}
				if _, v, ok := itr.Next(); !ok || v != w {
					t.Fatalf("element %d = %d, %v, want %d", i, v, ok, w)
				}
			}
			if !itr.Done() {
				t.Fatalf("Done() is false after the last element")
			}
		})
	}
}

// TestQueueBuilderMatchesEnqueue checks that a built queue behaves like one made
// by Enqueue, can be extended and drained, and does not share the caller's
// slices.
func TestQueueBuilderMatchesEnqueue(t *testing.T) {
	for _, n := range []int{0, 1, 2, 31, 32, 33, 1000} {
		want := make([]int, n)
		for i := range want {
			want[i] = i
		}

		b := NewQueueBuilder[int]()
		half := n / 2
		for _, v := range want[:half] {
			b.Enqueue(v)
		}
		chunk := slices.Clone(want[half:])
		b.EnqueueSlice(chunk)
		for i := range chunk {
			chunk[i] = -99 // the builder must have copied the values
		}
		if b.Len() != n {
			t.Fatalf("n=%d: builder Len() = %d", n, b.Len())
		}
		q := b.Queue()
		requireQueue(t, q, want)

		// Extend it, then drain it.
		q2 := q.Enqueue(n)
		requireQueue(t, q, want)
		requireQueue(t, q2, append(slices.Clone(want), n))
		for i := 0; !q2.Empty(); i++ {
			var v int
			q2, v, _ = q2.Dequeue()
			if v != i {
				t.Fatalf("n=%d: drain gave %d at position %d", n, v, i)
			}
		}
	}
}

// TestQueueNewQueueOfCopiesInput checks the queue does not alias the slice it
// was built from, for the constructor that takes a slice.
func TestQueueNewQueueOfCopiesInput(t *testing.T) {
	values := []int{1, 2, 3}
	q := NewQueueOf(values)
	values[0], values[2] = 99, 99
	requireQueue(t, q, []int{1, 2, 3})
}

// TestQueueAllocationContract pins the allocation counts that are the point of
// the memoized reversal and of a builder that fills the front. Allocation counts
// are exact, so unlike a timing they cannot be flaky.
func TestQueueAllocationContract(t *testing.T) {
	const n = 1000

	t.Run("ReadingABackOnlyQueueAgainAllocatesNothingMore", func(t *testing.T) {
		q := queueFromEnqueue(n)
		q.Peek() // the first read may prepare the queue
		if got := testing.AllocsPerRun(20, func() { sinkInt, sinkBool = q.Peek() }); got != 0 {
			t.Errorf("repeated Peek allocates %v times, want 0", got)
		}
		if got := testing.AllocsPerRun(20, func() { sinkAny, sinkInt, sinkBool = q.Dequeue() }); got != 1 {
			t.Errorf("repeated Dequeue allocates %v times, want 1 (the new Queue)", got)
		}
		if got := testing.AllocsPerRun(20, func() { sinkAny = q.Iterator() }); got != 1 {
			t.Errorf("repeated Iterator() allocates %v times, want 1 (the iterator)", got)
		}
	})

	t.Run("BuiltQueueNeedsNoReversal", func(t *testing.T) {
		const runs = 20
		queues := make([]*Queue[int], runs+1) // AllocsPerRun calls the function once extra
		for i := range queues {
			b := NewQueueBuilder[int]()
			for j := 0; j < n; j++ {
				b.Enqueue(j)
			}
			queues[i] = b.Queue()
		}
		i := 0
		if got := testing.AllocsPerRun(runs, func() {
			sinkAny, sinkInt, sinkBool = queues[i].Dequeue()
			i++
		}); got != 1 {
			t.Errorf("first Dequeue of a built queue allocates %v times, want 1 (the new Queue)", got)
		}
	})

	// Reading an empty queue has nothing to turn around, so it must not make a
	// cached copy. Each run uses a queue that has not been read before.
	t.Run("EmptyQueueIsNotCopiedOnRead", func(t *testing.T) {
		const runs = 20
		queues := make([]*Queue[int], runs+1)
		for i := range queues {
			queues[i] = NewQueue[int]()
		}
		i := 0
		if got := testing.AllocsPerRun(runs, func() {
			sinkAny = queues[i].Iterator()
			i++
		}); got != 1 {
			t.Errorf("Iterator() of an empty queue allocates %v times, want 1 (the iterator)", got)
		}
	})

	t.Run("NewQueueOfAllocatesOncePerElement", func(t *testing.T) {
		values := make([]int, n)
		got := testing.AllocsPerRun(5, func() { sinkAny = NewQueueOf(values) })
		if want := float64(n + 1); got != want { // one node per element, plus the Queue
			t.Errorf("NewQueueOf of %d values allocates %v times, want %v", n, got, want)
		}
	})
}
