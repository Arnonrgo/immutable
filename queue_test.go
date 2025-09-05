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
	// Dequeue 5 times to force front exhaustion and normalization from back
	for i := 0; i < 5; i++ {
		var ok bool
		q, _, ok = q.Dequeue()
		if !ok {
			t.Fatalf("unexpected empty during drain at %d", i)
		}
	}
	if !q.Empty() {
		t.Fatalf("expected empty after full drain")
	}
}
