package immutable

import (
	"fmt"
	"testing"
)

// Queue benchmarks. Names start with BenchmarkBaseline_Queue so
// scripts/bench.sh includes them in the baseline.
//
// A Queue keeps a front list and a reversed back list. A queue built by
// Enqueue has everything in the back list (front empty); a queue built by
// NewQueueOf has everything in the front list. The two shapes exercise
// different paths, so both are measured.

var benchQueueSizes = []int{100, 1000, 10000}

// queueFromEnqueue builds a queue with the persistent Enqueue loop.
// Everything ends up in the back list.
func queueFromEnqueue(n int) *Queue[int] {
	q := NewQueue[int]()
	for i := 0; i < n; i++ {
		q = q.Enqueue(i)
	}
	return q
}

// queueWithoutMemo returns a queue with the same lists as q and nothing cached,
// so reading it pays for any reversal again. It lets a benchmark measure the
// first read of a queue without building a new one every iteration.
func queueWithoutMemo(q *Queue[int]) *Queue[int] {
	return &Queue[int]{front: q.front, back: q.back}
}

// BenchmarkBaseline_Queue_Enqueue builds a queue with the persistent Enqueue loop.
func BenchmarkBaseline_Queue_Enqueue(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sinkAny = queueFromEnqueue(n)
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_EnqueueOne measures a single Enqueue on a queue of
// size N. The result is discarded, so the queue never grows.
func BenchmarkBaseline_Queue_EnqueueOne(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkAny = q.Enqueue(i)
			}
		})
	}
}

// BenchmarkBaseline_Queue_Builder builds a queue with QueueBuilder.
func BenchmarkBaseline_Queue_Builder(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				builder := NewQueueBuilder[int]()
				for j := 0; j < n; j++ {
					builder.Enqueue(j)
				}
				sinkAny = builder.Queue()
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_New builds a queue from an existing slice with NewQueueOf.
func BenchmarkBaseline_Queue_New(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			values := benchIntKeys(n, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkAny = NewQueueOf(values)
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_DrainFromEnqueue dequeues every element of a queue
// built by Enqueue (back list only). This is the linear-use case: one reversal,
// then O(1) per Dequeue. Each iteration starts from a queue that has not been
// read before, so the reversal is paid every time.
func BenchmarkBaseline_Queue_DrainFromEnqueue(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			base := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q := queueWithoutMemo(base)
				for !q.Empty() {
					var v int
					q, v, _ = q.Dequeue()
					sinkInt = v
				}
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_DrainFromSlice dequeues every element of a queue
// built by NewQueueOf (front list only, so no reversal is needed).
func BenchmarkBaseline_Queue_DrainFromSlice(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			base := NewQueueOf(benchIntKeys(n, false))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q := base
				for !q.Empty() {
					var v int
					q, v, _ = q.Dequeue()
					sinkInt = v
				}
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_PeekBackOnly measures Peek on a queue whose front is
// empty (built by Enqueue). The same immutable value is peeked repeatedly, so
// if the reversal is not memoized every call pays O(N). This is the
// persistent-reuse case the README's "amortized O(1)" claim does not cover.
func BenchmarkBaseline_Queue_PeekBackOnly(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkInt, sinkBool = q.Peek()
			}
		})
	}
}

// BenchmarkBaseline_Queue_PeekFront measures Peek on a queue whose front is
// populated (built by NewQueueOf): the O(1) case.
func BenchmarkBaseline_Queue_PeekFront(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := NewQueueOf(benchIntKeys(n, false))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkInt, sinkBool = q.Peek()
			}
		})
	}
}

// BenchmarkBaseline_Queue_DequeueBackOnly measures one Dequeue on a queue whose
// front is empty. The same value is dequeued repeatedly (the result is
// discarded), so a non-memoized reversal costs O(N) every call.
func BenchmarkBaseline_Queue_DequeueBackOnly(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				next, v, _ := q.Dequeue()
				sinkAny, sinkInt = next, v
			}
		})
	}
}

// BenchmarkBaseline_Queue_DequeueFront measures one Dequeue on a queue whose
// front is populated: the O(1) case.
func BenchmarkBaseline_Queue_DequeueFront(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := NewQueueOf(benchIntKeys(n, false))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				next, v, _ := q.Dequeue()
				sinkAny, sinkInt = next, v
			}
		})
	}
}

// BenchmarkBaseline_Queue_Iterate iterates a whole queue built by Enqueue
// (back list only), starting from a queue that has not been read before.
func BenchmarkBaseline_Queue_Iterate(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			base := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				itr := queueWithoutMemo(base).Iterator()
				for !itr.Done() {
					_, v, _ := itr.Next()
					sinkInt = v
				}
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_IterateReuse iterates the same back-only queue value
// over and over: the persistent-reuse case.
func BenchmarkBaseline_Queue_IterateReuse(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				itr := q.Iterator()
				for !itr.Done() {
					_, v, _ := itr.Next()
					sinkInt = v
				}
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_PeekFirstRead measures the first Peek of a back-only
// queue, which has to turn the back list around. This is the O(n) cost that
// remains; PeekBackOnly measures every read after it.
func BenchmarkBaseline_Queue_PeekFirstRead(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			base := queueFromEnqueue(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkInt, sinkBool = queueWithoutMemo(base).Peek()
			}
		})
	}
}

// BenchmarkBaseline_Queue_IterateMixed iterates a queue that has both lists
// populated: half the elements came from NewQueueOf (front), half from Enqueue
// (back).
func BenchmarkBaseline_Queue_IterateMixed(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			q := NewQueueOf(benchIntKeys(n/2, false))
			for i := n / 2; i < n; i++ {
				q = q.Enqueue(i)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				itr := q.Iterator()
				for !itr.Done() {
					_, v, _ := itr.Next()
					sinkInt = v
				}
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_Queue_BuildAndDrain builds a queue with QueueBuilder and
// dequeues every element: the whole life of a work queue that is filled in one
// go and then consumed. A builder that leaves everything in the back list pays
// a reversal at the first Dequeue; one that fills the front does not.
func BenchmarkBaseline_Queue_BuildAndDrain(b *testing.B) {
	for _, n := range benchQueueSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				builder := NewQueueBuilder[int]()
				for j := 0; j < n; j++ {
					builder.Enqueue(j)
				}
				q := builder.Queue()
				for !q.Empty() {
					var v int
					q, v, _ = q.Dequeue()
					sinkInt = v
				}
			}
			reportPerElem(b, n)
		})
	}
}
