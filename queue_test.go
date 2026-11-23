package immutable

import "testing"

func TestQueueBasic(t *testing.T) {
	q := NewQueue[int]()
	if !q.Empty() || q.Len() != 0 {
		t.Fatalf("expected empty queue")
	}

	q = q.Enqueue(1).Enqueue(2).Enqueue(3)
	if q.Len() != 3 {
		t.Fatalf("expected len=3, got %d", q.Len())
	}

	if v, ok := q.Peek(); !ok || v != 1 {
		t.Fatalf("peek expected 1, got %v ok=%v", v, ok)
	}

	q2, v, ok := q.Dequeue()
	if !ok || v != 1 {
		t.Fatalf("dequeue expected 1, got %v ok=%v", v, ok)
	}
	if q2.Len() != 2 {
		t.Fatalf("expected len=2, got %d", q2.Len())
	}

	q3, v, ok := q2.Dequeue()
	if !ok || v != 2 {
		t.Fatalf("dequeue expected 2, got %v ok=%v", v, ok)
	}
	q4, v, ok := q3.Dequeue()
	if !ok || v != 3 {
		t.Fatalf("dequeue expected 3, got %v ok=%v", v, ok)
	}
	if !q4.Empty() || q4.Len() != 0 {
		t.Fatalf("expected empty after draining")
	}
}

func TestQueueIteratorOrder(t *testing.T) {
	q := NewQueue[int]()
	for i := 0; i < 10; i++ {
		q = q.Enqueue(i)
	}

	itr := q.Iterator()
	count := 0
	for !itr.Done() {
		idx, v, ok := itr.Next()
		if !ok {
			t.Fatalf("iterator prematurely ended")
		}
		if idx != count || v != count {
			t.Fatalf("expected idx=%d v=%d, got idx=%d v=%d", count, count, idx, v)
		}
		count++
	}
	if count != 10 {
		t.Fatalf("expected to iterate 10, got %d", count)
	}
}

func TestQueueNormalizeBoundary(t *testing.T) {
	q := NewQueue[int]()
	for i := 0; i < 5; i++ {
		q = q.Enqueue(i)
	}
	for i := 0; i < 5; i++ {
		var v int
		var ok bool
		q, v, ok = q.Dequeue()
		if !ok || v != i {
			t.Fatalf("Dequeue %d = %d, %v; want %d, true", i, v, ok, i)
		}
	}
	if !q.Empty() {
		t.Fatalf("expected empty after full drain")
	}
}

func TestQueueMixedOps(t *testing.T) {
	q := NewQueue[int]()

	// Enqueue 1..5, dequeue 1..3, enqueue 6..8, drain remaining
	for i := 1; i <= 5; i++ {
		q = q.Enqueue(i)
	}
	for i := 1; i <= 3; i++ {
		var v int
		var ok bool
		q, v, ok = q.Dequeue()
		if !ok || v != i {
			t.Fatalf("dequeue expected %d, got %d ok=%v", i, v, ok)
		}
	}
	for i := 6; i <= 8; i++ {
		q = q.Enqueue(i)
	}
	// Should now contain 4, 5, 6, 7, 8
	expected := []int{4, 5, 6, 7, 8}
	if q.Len() != len(expected) {
		t.Fatalf("expected len=%d, got %d", len(expected), q.Len())
	}
	for i, want := range expected {
		var v int
		var ok bool
		q, v, ok = q.Dequeue()
		if !ok || v != want {
			t.Fatalf("position %d: expected %d, got %d ok=%v", i, want, v, ok)
		}
	}
	if !q.Empty() {
		t.Fatalf("expected empty after full drain")
	}
}

func TestQueueSnapshotIsolation(t *testing.T) {
	q0 := NewQueue[int]()
	q1 := q0.Enqueue(1).Enqueue(2).Enqueue(3)

	// q2 dequeues from q1 — q1 must remain unchanged
	q2, v, ok := q1.Dequeue()
	if !ok || v != 1 {
		t.Fatalf("dequeue expected 1, got %d", v)
	}

	// q1 snapshot: still has 1, 2, 3
	if q1.Len() != 3 {
		t.Fatalf("q1 snapshot len expected 3, got %d", q1.Len())
	}
	if v, ok := q1.Peek(); !ok || v != 1 {
		t.Fatalf("q1 snapshot peek expected 1, got %d", v)
	}

	// q2 snapshot: has 2, 3
	if q2.Len() != 2 {
		t.Fatalf("q2 snapshot len expected 2, got %d", q2.Len())
	}
	if v, ok := q2.Peek(); !ok || v != 2 {
		t.Fatalf("q2 snapshot peek expected 2, got %d", v)
	}

	// q0 still empty
	if !q0.Empty() {
		t.Fatalf("q0 snapshot should still be empty")
	}

	// Enqueue onto q2 creates q3, doesn't affect q1 or q2
	q3 := q2.Enqueue(99)
	if q2.Len() != 2 {
		t.Fatalf("q2 should be unchanged after enqueue, got len %d", q2.Len())
	}
	if q3.Len() != 3 {
		t.Fatalf("q3 expected len 3, got %d", q3.Len())
	}
}

func TestQueueMixedIterator(t *testing.T) {
	q := NewQueue[int]()
	for i := 1; i <= 5; i++ {
		q = q.Enqueue(i)
	}
	// Dequeue 2, then enqueue 2 more — creates mixed front/back state
	q, _, _ = q.Dequeue()
	q, _, _ = q.Dequeue()
	q = q.Enqueue(6)
	q = q.Enqueue(7)

	// Should iterate: 3, 4, 5, 6, 7
	expected := []int{3, 4, 5, 6, 7}
	itr := q.Iterator()
	for i, want := range expected {
		idx, v, ok := itr.Next()
		if !ok {
			t.Fatalf("iterator ended early at position %d", i)
		}
		if idx != i || v != want {
			t.Fatalf("position %d: expected idx=%d v=%d, got idx=%d v=%d", i, i, want, idx, v)
		}
	}
	if !itr.Done() {
		t.Fatalf("iterator should be done")
	}
}

func TestQueueBuilder(t *testing.T) {
	b := NewQueueBuilder[int]()
	b.Enqueue(10)
	b.EnqueueSlice([]int{20, 30})

	if b.Len() != 3 {
		t.Fatalf("expected builder len 3, got %d", b.Len())
	}

	q := b.Queue()
	expected := []int{10, 20, 30}
	for _, want := range expected {
		var v int
		var ok bool
		q, v, ok = q.Dequeue()
		if !ok || v != want {
			t.Fatalf("expected %d, got %d ok=%v", want, v, ok)
		}
	}
}

func TestQueueEmptyOps(t *testing.T) {
	var q *Queue[int]
	if q.Len() != 0 || !q.Empty() {
		t.Fatalf("nil queue should be empty")
	}
	if _, ok := q.Peek(); ok {
		t.Fatalf("peek on nil should return !ok")
	}
	if _, _, ok := q.Dequeue(); ok {
		t.Fatalf("dequeue on nil should return !ok")
	}
	q2 := q.Enqueue(1)
	if q2.Len() != 1 {
		t.Fatalf("enqueue on nil should produce len=1, got %d", q2.Len())
	}
}
