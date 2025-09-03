package immutable

import (
	"fmt"
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
		if builder.list != nil {
			t.Error("Builder should be invalidated after List() call")
		}
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
		entries := []mapEntry[int, string]{
			{1, "one"}, {2, "two"}, {3, "three"}, {4, "four"}, {5, "five"},
		}

		// Filter only even keys
		builder.Filter(entries, func(k int, v string) bool { return k%2 == 0 })
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
