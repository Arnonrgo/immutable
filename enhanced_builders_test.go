package immutable

import (
	"fmt"
	"maps"
	"testing"
)

// TestBatchListBuilder tests batch list construction
func TestBatchListBuilder(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		builder := NewBatchListBuilder[int](3) // Small batch size for testing

		// Add some values
		builder.Append(1)
		builder.Append(2)
		builder.Append(3) // This should trigger a flush
		builder.Append(4)
		builder.Append(5)

		// Check length before final flush
		if got := builder.Len(); got != 5 {
			t.Errorf("Expected length 5, got %d", got)
		}

		// Get final list
		list := builder.List()
		if list.Len() != 5 {
			t.Errorf("Expected final list length 5, got %d", list.Len())
		}

		// Verify contents
		for i := 0; i < 5; i++ {
			if got := list.Get(i); got != i+1 {
				t.Errorf("Expected list[%d] = %d, got %d", i, i+1, got)
			}
		}

		// Builder should be invalidated
		mustPanic(t, "Append after List", func() { builder.Append(6) })
		mustPanic(t, "second List", func() { builder.List() })
	})

	t.Run("AppendSlice", func(t *testing.T) {
		builder := NewBatchListBuilder[string](5)
		values := []string{"a", "b", "c", "d", "e", "f", "g"}

		builder.AppendSlice(values)
		list := builder.List()

		if list.Len() != len(values) {
			t.Errorf("Expected length %d, got %d", len(values), list.Len())
		}

		for i, expected := range values {
			if got := list.Get(i); got != expected {
				t.Errorf("Expected list[%d] = %s, got %s", i, expected, got)
			}
		}
	})

	t.Run("EmptyBuilder", func(t *testing.T) {
		builder := NewBatchListBuilder[int](10)
		list := builder.List()

		if list.Len() != 0 {
			t.Errorf("Expected empty list, got length %d", list.Len())
		}
	})
}

// TestBatchMapBuilder tests batch map construction
func TestBatchMapBuilder(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		builder := NewBatchMapBuilder[int, string](nil, 3)

		// Add some entries
		builder.Set(1, "one")
		builder.Set(2, "two")
		builder.Set(3, "three") // Should trigger flush
		builder.Set(4, "four")

		if got := builder.Len(); got != 4 {
			t.Errorf("Expected length 4, got %d", got)
		}

		// Get final map
		m := builder.Map()
		if m.Len() != 4 {
			t.Errorf("Expected final map length 4, got %d", m.Len())
		}

		// Verify contents
		expected := map[int]string{1: "one", 2: "two", 3: "three", 4: "four"}
		for key, expectedValue := range expected {
			if got, ok := m.Get(key); !ok || got != expectedValue {
				t.Errorf("Expected m[%d] = %s, got %s (exists: %v)", key, expectedValue, got, ok)
			}
		}
	})

	t.Run("SetMap", func(t *testing.T) {
		builder := NewBatchMapBuilder[string, int](nil, 5)
		entries := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

		builder.SetMap(entries)
		m := builder.Map()

		if m.Len() != len(entries) {
			t.Errorf("Expected length %d, got %d", len(entries), m.Len())
		}

		for key, expectedValue := range entries {
			if got, ok := m.Get(key); !ok || got != expectedValue {
				t.Errorf("Expected m[%s] = %d, got %d (exists: %v)", key, expectedValue, got, ok)
			}
		}
	})
}

// TestBatchSetBuilder tests batch set construction
func TestBatchSetBuilder(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		builder := NewBatchSetBuilder[int](nil, 3)

		// Add some values
		builder.Add(1)
		builder.Add(2)
		builder.Add(3) // Should trigger flush
		builder.Add(4)
		builder.Add(2) // Duplicate should be ignored

		// Note: Len() might include duplicates in buffer before final flush
		// The important test is the final set length

		// Get final set
		set := builder.Set()
		if set.Len() != 4 {
			t.Errorf("Expected final set length 4, got %d", set.Len())
		}

		// Verify contents
		expected := []int{1, 2, 3, 4}
		for _, value := range expected {
			if !set.Has(value) {
				t.Errorf("Expected set to contain %d", value)
			}
		}
	})

	t.Run("AddSlice", func(t *testing.T) {
		builder := NewBatchSetBuilder[string](nil, 5)
		values := []string{"a", "b", "c", "b", "d"} // "b" is duplicate

		builder.AddSlice(values)
		set := builder.Set()

		expectedLen := 4 // unique values
		if set.Len() != expectedLen {
			t.Errorf("Expected length %d, got %d", expectedLen, set.Len())
		}

		unique := []string{"a", "b", "c", "d"}
		for _, value := range unique {
			if !set.Has(value) {
				t.Errorf("Expected set to contain %s", value)
			}
		}
	})
}

// TestStreamingListBuilder tests streaming list operations
func TestStreamingListBuilder(t *testing.T) {
	t.Run("AutoFlush", func(t *testing.T) {
		builder := NewStreamingListBuilder[int](3, 6) // autoFlush at 6

		// Add values without reaching auto-flush threshold
		for i := 1; i <= 5; i++ {
			builder.Append(i)
		}

		// Should not have auto-flushed yet
		if builder.Len() != 5 {
			t.Errorf("Expected length 5 before auto-flush, got %d", builder.Len())
		}

		// Add one more to trigger auto-flush
		builder.Append(6)

		list := builder.List()
		if list.Len() != 6 {
			t.Errorf("Expected final length 6, got %d", list.Len())
		}
	})

	t.Run("Filter", func(t *testing.T) {
		builder := NewStreamingListBuilder[int](5, 0)
		values := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		// Filter only even numbers
		builder.Filter(values, func(x int) bool { return x%2 == 0 })
		list := builder.List()

		expectedLen := 5 // 2, 4, 6, 8, 10
		if list.Len() != expectedLen {
			t.Errorf("Expected length %d, got %d", expectedLen, list.Len())
		}

		// Verify all values are even
		for i := 0; i < list.Len(); i++ {
			if value := list.Get(i); value%2 != 0 {
				t.Errorf("Expected even number, got %d", value)
			}
		}
	})

	t.Run("Transform", func(t *testing.T) {
		builder := NewStreamingListBuilder[int](5, 0)
		values := []int{1, 2, 3, 4, 5}

		// Transform by doubling
		builder.Transform(values, func(x int) int { return x * 2 })
		list := builder.List()

		if list.Len() != len(values) {
			t.Errorf("Expected length %d, got %d", len(values), list.Len())
		}

		for i, original := range values {
			expected := original * 2
			if got := list.Get(i); got != expected {
				t.Errorf("Expected list[%d] = %d, got %d", i, expected, got)
			}
		}
	})
}

// TestStreamingMapBuilder tests streaming map operations
func TestStreamingMapBuilder(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		builder := NewStreamingMapBuilder[int, string](nil, 3, 6)

		for i := 1; i <= 5; i++ {
			builder.Set(i, fmt.Sprintf("value-%d", i))
		}

		m := builder.Map()
		if m.Len() != 5 {
			t.Errorf("Expected length 5, got %d", m.Len())
		}

		for i := 1; i <= 5; i++ {
			expected := fmt.Sprintf("value-%d", i)
			if got, ok := m.Get(i); !ok || got != expected {
				t.Errorf("Expected m[%d] = %s, got %s (exists: %v)", i, expected, got, ok)
			}
		}
	})

	t.Run("SetMany", func(t *testing.T) {
		builder := NewStreamingMapBuilder[string, int](nil, 2, 0)
		entries := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

		builder.SetMany(entries)
		m := builder.Map()

		if m.Len() != len(entries) {
			t.Errorf("Expected length %d, got %d", len(entries), m.Len())
		}

		for key, expectedValue := range entries {
			if got, ok := m.Get(key); !ok || got != expectedValue {
				t.Errorf("Expected m[%s] = %d, got %d (exists: %v)", key, expectedValue, got, ok)
			}
		}
	})

	t.Run("Filter", func(t *testing.T) {
		builder := NewStreamingMapBuilder[int, string](nil, 5, 0)
		entries := map[int]string{1: "one", 2: "two", 3: "three", 4: "four", 5: "five"}

		builder.Filter(maps.All(entries), func(k int, v string) bool { return k%2 == 0 })
		m := builder.Map()

		expectedLen := 2 // keys 2 and 4
		if m.Len() != expectedLen {
			t.Errorf("Expected length %d, got %d", expectedLen, m.Len())
		}

		if got, ok := m.Get(2); !ok || got != "two" {
			t.Errorf("Expected m[2] = two, got %s (exists: %v)", got, ok)
		}
		if got, ok := m.Get(4); !ok || got != "four" {
			t.Errorf("Expected m[4] = four, got %s (exists: %v)", got, ok)
		}
	})

	t.Run("Stream", func(t *testing.T) {
		builder := NewStreamingMapBuilder[string, int](nil, 3, 6)
		entries := map[string]int{"a": 1, "b": 2, "c": 3}

		builder.Stream(maps.All(entries))
		m := builder.Map()

		if m.Len() != 3 {
			t.Errorf("Expected length 3, got %d", m.Len())
		}
		for k, v := range entries {
			if got, ok := m.Get(k); !ok || got != v {
				t.Errorf("Expected m[%s] = %d, got %d (exists: %v)", k, v, got, ok)
			}
		}
	})

	t.Run("Transform", func(t *testing.T) {
		builder := NewStreamingMapBuilder[int, int](nil, 5, 0)
		entries := map[int]int{1: 10, 2: 20, 3: 30}

		builder.Transform(maps.All(entries), func(k, v int) (int, int) { return k * 10, v * 2 })
		m := builder.Map()

		if m.Len() != 3 {
			t.Errorf("Expected length 3, got %d", m.Len())
		}
		expected := map[int]int{10: 20, 20: 40, 30: 60}
		for k, v := range expected {
			if got, ok := m.Get(k); !ok || got != v {
				t.Errorf("Expected m[%d] = %d, got %d (exists: %v)", k, v, got, ok)
			}
		}
	})
}

// TestSortedBatchBuilder tests sorted batch operations
func TestSortedBatchBuilder(t *testing.T) {
	t.Run("SortedBuffer", func(t *testing.T) {
		builder := NewSortedBatchBuilder[int, string](nil, 3, true) // maintain sort

		// Add in random order
		builder.Set(3, "three")
		builder.Set(1, "one")
		builder.Set(2, "two")
		builder.Set(5, "five")
		builder.Set(4, "four")

		sm := builder.SortedMap()
		if sm.Len() != 5 {
			t.Errorf("Expected length 5, got %d", sm.Len())
		}

		// Verify sorted iteration
		itr := sm.Iterator()
		expectedKeys := []int{1, 2, 3, 4, 5}
		i := 0
		for !itr.Done() {
			key, _, _ := itr.Next()
			if key != expectedKeys[i] {
				t.Errorf("Expected key %d at position %d, got %d", expectedKeys[i], i, key)
			}
			i++
		}
	})

	t.Run("UnsortedBuffer", func(t *testing.T) {
		builder := NewSortedBatchBuilder[int, string](nil, 5, false) // don't maintain sort

		for i := 5; i >= 1; i-- {
			builder.Set(i, fmt.Sprintf("value-%d", i))
		}

		sm := builder.SortedMap()
		if sm.Len() != 5 {
			t.Errorf("Expected length 5, got %d", sm.Len())
		}

		// Should still be sorted in final map
		itr := sm.Iterator()
		expectedKeys := []int{1, 2, 3, 4, 5}
		i := 0
		for !itr.Done() {
			key, _, _ := itr.Next()
			if key != expectedKeys[i] {
				t.Errorf("Expected key %d at position %d, got %d", expectedKeys[i], i, key)
			}
			i++
		}
	})
}

// TestBatchSortedSetBuilder tests batch sorted set construction
func TestBatchSortedSetBuilder(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		builder := NewBatchSortedSetBuilder[int](nil, 3, true)

		// Add in random order
		values := []int{5, 2, 8, 1, 9, 3}
		for _, value := range values {
			builder.Add(value)
		}

		set := builder.SortedSet()
		if set.Len() != len(values) {
			t.Errorf("Expected length %d, got %d", len(values), set.Len())
		}

		// Verify sorted iteration
		itr := set.Iterator()
		expectedValues := []int{1, 2, 3, 5, 8, 9}
		i := 0
		for !itr.Done() {
			value, _ := itr.Next()
			if value != expectedValues[i] {
				t.Errorf("Expected value %d at position %d, got %d", expectedValues[i], i, value)
			}
			i++
		}
	})

	t.Run("AddSlice", func(t *testing.T) {
		builder := NewBatchSortedSetBuilder[string](nil, 5, false)
		values := []string{"zebra", "apple", "banana", "apple", "cherry"} // "apple" is duplicate

		builder.AddSlice(values)
		set := builder.SortedSet()

		expectedLen := 4 // unique values
		if set.Len() != expectedLen {
			t.Errorf("Expected length %d, got %d", expectedLen, set.Len())
		}

		// Verify sorted order
		itr := set.Iterator()
		expectedOrder := []string{"apple", "banana", "cherry", "zebra"}
		i := 0
		for !itr.Done() {
			value, _ := itr.Next()
			if value != expectedOrder[i] {
				t.Errorf("Expected value %s at position %d, got %s", expectedOrder[i], i, value)
			}
			i++
		}
	})
}

// TestBatchMapBuilder_LastWriteWins verifies that duplicate keys in a buffer
// resolve to the last value written, regardless of buffer size or map state.
func TestBatchMapBuilder_LastWriteWins(t *testing.T) {
	t.Run("SmallBuffer_EmptyMap", func(t *testing.T) {
		builder := NewBatchMapBuilder[int, string](nil, 64)
		builder.Set(1, "first")
		builder.Set(2, "two")
		builder.Set(1, "second")
		m := builder.Map()
		if v, _ := m.Get(1); v != "second" {
			t.Errorf("expected last-write-wins: got %q, want %q", v, "second")
		}
	})

	t.Run("LargeBuffer_EmptyMap", func(t *testing.T) {
		builder := NewBatchMapBuilder[int, string](nil, 64)
		for i := 0; i < 12; i++ {
			builder.Set(i, "first")
		}
		builder.Set(0, "overwrite")
		m := builder.Map()
		if v, _ := m.Get(0); v != "overwrite" {
			t.Errorf("expected last-write-wins: got %q, want %q", v, "overwrite")
		}
		if m.Len() != 12 {
			t.Errorf("expected 12 entries, got %d", m.Len())
		}
	})

	t.Run("SmallBuffer_NonEmptyArrayNode", func(t *testing.T) {
		builder := NewBatchMapBuilder[int, string](nil, 64)
		builder.Set(1, "existing")
		builder.Flush()
		// Now the map has a non-empty array node; flush a small buffer with dups
		builder.Set(2, "two")
		builder.Set(1, "updated-1")
		builder.Set(2, "updated-2")
		m := builder.Map()
		if v, _ := m.Get(1); v != "updated-1" {
			t.Errorf("expected last-write-wins: got %q, want %q", v, "updated-1")
		}
		if v, _ := m.Get(2); v != "updated-2" {
			t.Errorf("expected last-write-wins: got %q, want %q", v, "updated-2")
		}
	})

	t.Run("LargeBuffer_NonEmptyArrayNode", func(t *testing.T) {
		builder := NewBatchMapBuilder[int, string](nil, 64)
		builder.Set(1, "existing")
		builder.Flush()
		// Now flush a large buffer (> maxArrayMapSize=8) with duplicate keys
		for i := 0; i < 10; i++ {
			builder.Set(i, fmt.Sprintf("first-%d", i))
		}
		builder.Set(1, "overwrite-1")
		builder.Set(5, "overwrite-5")
		m := builder.Map()
		if v, _ := m.Get(1); v != "overwrite-1" {
			t.Errorf("expected last-write-wins: got %q, want %q", v, "overwrite-1")
		}
		if v, _ := m.Get(5); v != "overwrite-5" {
			t.Errorf("expected last-write-wins: got %q, want %q", v, "overwrite-5")
		}
		if v, _ := m.Get(0); v != "first-0" {
			t.Errorf("expected non-overwritten key preserved: got %q, want %q", v, "first-0")
		}
	})
}

// TestBatchBuilderEdgeCases tests edge cases and error conditions
func TestBatchBuilderEdgeCases(t *testing.T) {
	t.Run("ZeroBatchSize", func(t *testing.T) {
		// Should use default batch size
		listBuilder := NewBatchListBuilder[int](0)
		listBuilder.Append(1)
		list := listBuilder.List()
		if list.Len() != 1 {
			t.Errorf("Expected length 1, got %d", list.Len())
		}

		mapBuilder := NewBatchMapBuilder[int, string](nil, -1)
		mapBuilder.Set(1, "one")
		m := mapBuilder.Map()
		if m.Len() != 1 {
			t.Errorf("Expected length 1, got %d", m.Len())
		}
	})

	t.Run("MultipleFlushes", func(t *testing.T) {
		builder := NewBatchListBuilder[int](10)
		builder.Append(1)
		builder.Flush()
		builder.Append(2)
		builder.Flush()
		builder.Flush() // Multiple flushes should be safe

		list := builder.List()
		if list.Len() != 2 {
			t.Errorf("Expected length 2, got %d", list.Len())
		}
	})

	t.Run("BuilderReuse", func(t *testing.T) {
		builder := NewBatchMapBuilder[int, string](nil, 5)
		builder.Set(1, "one")

		m1 := builder.Map()
		m2 := builder.Map() // Should return nil after first call

		if m1.Len() != 1 {
			t.Errorf("Expected first map length 1, got %d", m1.Len())
		}
		if m2 != nil {
			t.Error("Expected second map to be nil after builder invalidation")
		}
	})
}

// mustPanic fails the test unless f panics.
func mustPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		if recover() == nil {
			t.Errorf("%s: expected a panic", what)
		}
	}()
	f()
}

// requireListEquals checks Len, Get and the iterator of l against want.
func requireListEquals(t *testing.T, l *List[int], want []int) {
	t.Helper()
	if l.Len() != len(want) {
		t.Fatalf("Len() = %d, want %d", l.Len(), len(want))
	}
	for i, w := range want {
		if got := l.Get(i); got != w {
			t.Fatalf("Get(%d) = %d, want %d", i, got, w)
		}
	}
	itr := l.Iterator()
	for i := 0; !itr.Done(); i++ {
		if idx, v := itr.Next(); idx != i || v != want[i] {
			t.Fatalf("iterator at %d returned (%d, %d), want (%d, %d)", i, idx, v, i, want[i])
		}
	}
}

// TestBatchListBuilder_Lifecycle covers the states of the builder: valid,
// invalid after List, valid again after Reset.
func TestBatchListBuilder_Lifecycle(t *testing.T) {
	t.Run("InvalidAfterList", func(t *testing.T) {
		b := NewBatchListBuilder[int](0)
		b.Append(1)
		_ = b.List()
		mustPanic(t, "Append", func() { b.Append(2) })
		mustPanic(t, "AppendSlice", func() { b.AppendSlice([]int{2}) })
		mustPanic(t, "Len", func() { b.Len() })
		mustPanic(t, "List", func() { b.List() })
	})

	t.Run("ResetMakesBuilderValidAndEmpty", func(t *testing.T) {
		b := NewBatchListBuilder[int](4)
		b.AppendSlice([]int{1, 2, 3})
		first := b.List()
		b.Reset()
		if b.Len() != 0 {
			t.Fatalf("Len() after Reset = %d, want 0", b.Len())
		}
		b.Append(9)
		second := b.List()
		requireListEquals(t, first, []int{1, 2, 3})
		requireListEquals(t, second, []int{9})
	})

	t.Run("ResetBeforeList", func(t *testing.T) {
		b := NewBatchListBuilder[int](4)
		b.AppendSlice([]int{1, 2, 3})
		b.Reset()
		b.Append(7)
		requireListEquals(t, b.List(), []int{7})
	})

	t.Run("FlushIsNoOp", func(t *testing.T) {
		b := NewBatchListBuilder[int](2)
		b.Append(1)
		b.Flush()
		b.Append(2)
		b.Flush()
		if b.Len() != 2 {
			t.Fatalf("Len() = %d, want 2", b.Len())
		}
		requireListEquals(t, b.List(), []int{1, 2})
	})
}

// TestBatchListBuilder_NoAliasing checks that a list built by the builder owns
// its memory: neither later builder use nor changes to the caller's slice can
// show through. Both representations are covered (slice node and trie).
func TestBatchListBuilder_NoAliasing(t *testing.T) {
	for _, n := range []int{5, listSliceThreshold, listSliceThreshold + 1, 3 * listNodeSize * listNodeSize} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			want := make([]int, n)
			for i := range want {
				want[i] = i
			}

			// The caller's slice is not retained by AppendSlice.
			input := append([]int(nil), want...)
			b := NewBatchListBuilder[int](0)
			b.AppendSlice(input)
			for i := range input {
				input[i] = -1
			}
			list := b.List()
			requireListEquals(t, list, want)

			// Reusing the builder after Reset must not change the list.
			b.Reset()
			for i := 0; i < n; i++ {
				b.Append(-2)
			}
			_ = b.List()
			requireListEquals(t, list, want)
		})
	}
}

// TestBatchListBuilder_Sizes builds lists of sizes around every node-capacity
// boundary and checks them against a reference slice, using each of the three
// ways to add values.
func TestBatchListBuilder_Sizes(t *testing.T) {
	sizes := []int{0, 1, 2, 31, 32, 33, 63, 64, 65, 1023, 1024, 1025, 2048, 32767, 32768, 32769}
	for _, n := range sizes {
		want := make([]int, n)
		for i := range want {
			want[i] = i * 3
		}
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			t.Run("Append", func(t *testing.T) {
				b := NewBatchListBuilder[int](16)
				for _, v := range want {
					b.Append(v)
				}
				if b.Len() != n {
					t.Fatalf("Len() = %d, want %d", b.Len(), n)
				}
				l := b.List()
				requireListEquals(t, l, want)
				requireTrieShape(t, l)
			})
			t.Run("AppendSlice", func(t *testing.T) {
				b := NewBatchListBuilder[int](16)
				b.AppendSlice(want)
				l := b.List()
				requireListEquals(t, l, want)
				requireTrieShape(t, l)
			})
			t.Run("AppendSliceInPieces", func(t *testing.T) {
				b := NewBatchListBuilder[int](1)
				for i := 0; i < n; i += 7 {
					b.AppendSlice(want[i:min(i+7, n)])
				}
				l := b.List()
				requireListEquals(t, l, want)
				requireTrieShape(t, l)
			})
			t.Run("MixedAppendAndAppendSlice", func(t *testing.T) {
				// Alternating single values and slices of awkward length makes
				// slice copies start and end at every offset inside a leaf.
				b := NewBatchListBuilder[int](0)
				for i := 0; i < n; {
					b.Append(want[i])
					i++
					k := min(i+(i%37), n)
					b.AppendSlice(want[i:k])
					i = k
				}
				l := b.List()
				requireListEquals(t, l, want)
				requireTrieShape(t, l)
				// Appending after the build is a normal persistent operation.
				requireListEquals(t, l.Append(-1), append(append([]int(nil), want...), -1))
			})
		})
	}
}
