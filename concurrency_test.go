package immutable

import (
	"sync"
	"testing"
)

// Test that copy-on-write isolation holds for Map across concurrent readers.
func TestMap_CopyOnWriteIsolation(t *testing.T) {
	m1 := NewMap[int, int](nil)
	for i := 0; i < 1000; i++ {
		m1 = m1.Set(i, i*2)
	}
	m2 := m1.Set(0, 9999)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			v, ok := m1.Get(0)
			if !ok || v != 0 {
				t.Errorf("m1 expected key 0 => 0, got %v (ok=%v)", v, ok)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			v, ok := m2.Get(0)
			if !ok || v != 9999 {
				t.Errorf("m2 expected key 0 => 9999, got %v (ok=%v)", v, ok)
				return
			}
		}
	}()

	wg.Wait()
}

// Test that concurrent readers observe consistent values.
func TestMap_ConcurrentReaders(t *testing.T) {
	m := NewMap[int, int](nil)
	for i := 0; i < 10000; i++ {
		m = m.Set(i, i*2)
	}

	var wg sync.WaitGroup
	g := 8
	wg.Add(g)
	for j := 0; j < g; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 20000; i++ {
				v, ok := m.Get(i % 10000)
				if !ok || v != (i%10000)*2 {
					t.Errorf("expected %d, got %v (ok=%v)", (i%10000)*2, v, ok)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// Test that copy-on-write isolation holds for List across concurrent readers.
func TestList_CopyOnWriteIsolation(t *testing.T) {
	l1 := NewList[int]()
	for i := 0; i < 1000; i++ {
		l1 = l1.Append(i)
	}
	l2 := l1.Set(0, 9999)

	// Several readers share each list, so a read that writes shared state races.
	var wg sync.WaitGroup
	wg.Add(4)
	for r := 0; r < 2; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				if v := l1.Get(0); v != 0 {
					t.Errorf("l1 expected index 0 => 0, got %v", v)
					return
				}
			}
		}()

		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				if v := l2.Get(0); v != 9999 {
					t.Errorf("l2 expected index 0 => 9999, got %v", v)
					return
				}
			}
		}()
	}

	wg.Wait()
}
