package immutable

import (
	"testing"
)

// TestLazyCopyOnWrite verifies that the lazy copy-on-write optimization
// maintains immutability guarantees while improving performance
func TestLazyCopyOnWrite(t *testing.T) {
	// Create a map and add elements to trigger mapHashArrayNode usage
	m := NewMap[int, string](nil)

	// Add enough elements to create a mapHashArrayNode (> maxBitmapIndexedSize)
	for i := 0; i < 20; i++ {
		m = m.Set(i, "value"+string(rune('0'+i)))
	}

	// Create multiple versions by setting different values
	m1 := m.Set(100, "new_value_1")
	m2 := m.Set(101, "new_value_2")
	m3 := m1.Set(102, "new_value_3")

	// Verify immutability - original map should be unchanged
	if val, ok := m.Get(100); ok {
		t.Errorf("Original map should not contain key 100, but found value: %s", val)
	}
	if val, ok := m.Get(101); ok {
		t.Errorf("Original map should not contain key 101, but found value: %s", val)
	}

	// Verify each version has correct values
	if val, ok := m1.Get(100); !ok || val != "new_value_1" {
		t.Errorf("m1 should contain key 100 with value 'new_value_1', got: %s, %v", val, ok)
	}
	if val, ok := m1.Get(101); ok {
		t.Errorf("m1 should not contain key 101, but found value: %s", val)
	}

	if val, ok := m2.Get(101); !ok || val != "new_value_2" {
		t.Errorf("m2 should contain key 101 with value 'new_value_2', got: %s, %v", val, ok)
	}
	if val, ok := m2.Get(100); ok {
		t.Errorf("m2 should not contain key 100, but found value: %s", val)
	}

	if val, ok := m3.Get(100); !ok || val != "new_value_1" {
		t.Errorf("m3 should contain key 100 with value 'new_value_1', got: %s, %v", val, ok)
	}
	if val, ok := m3.Get(102); !ok || val != "new_value_3" {
		t.Errorf("m3 should contain key 102 with value 'new_value_3', got: %s, %v", val, ok)
	}

	// Verify all original values are still accessible
	for i := 0; i < 20; i++ {
		expectedVal := "value" + string(rune('0'+i))
		if val, ok := m.Get(i); !ok || val != expectedVal {
			t.Errorf("Original map should contain key %d with value '%s', got: %s, %v", i, expectedVal, val, ok)
		}
		if val, ok := m1.Get(i); !ok || val != expectedVal {
			t.Errorf("m1 should contain key %d with value '%s', got: %s, %v", i, expectedVal, val, ok)
		}
		if val, ok := m2.Get(i); !ok || val != expectedVal {
			t.Errorf("m2 should contain key %d with value '%s', got: %s, %v", i, expectedVal, val, ok)
		}
		if val, ok := m3.Get(i); !ok || val != expectedVal {
			t.Errorf("m3 should contain key %d with value '%s', got: %s, %v", i, expectedVal, val, ok)
		}
	}
}

// BenchmarkOptimizedMapSet compares the optimized map performance
func BenchmarkOptimizedMapSet(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run("size-"+string(rune('0'+size/1000)), func(b *testing.B) {
			// Pre-populate map
			m := NewMap[int, int](nil)
			for i := 0; i < size; i++ {
				m = m.Set(i, i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Set(i%size, i*2)
			}
		})
	}
}
