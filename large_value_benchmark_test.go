package immutable

import (
	"fmt"
	"testing"
)

// SmallValue represents a small value type
type SmallValue struct {
	ID int
}

// LargeValue represents a large value type (1KB)
type LargeValue struct {
	ID          int
	Name        string
	Description string
	Data        [200]int // ~800 bytes
	Metadata    [50]int  // ~200 bytes
	// Total: ~1KB
}

// HugeValue represents a very large value type (10KB)
type HugeValue struct {
	ID          int
	Name        string
	Description string
	Data        [2000]int // ~8KB
	Metadata    [500]int  // ~2KB
	// Total: ~10KB
}

func BenchmarkMapLargeValues(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	// Small values (8 bytes)
	for _, size := range sizes {
		b.Run(fmt.Sprintf("SmallValue/size-%d", size), func(b *testing.B) {
			m := NewMap[int, SmallValue](nil)
			for i := 0; i < size; i++ {
				m = m.Set(i, SmallValue{ID: i})
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Set(i%size, SmallValue{ID: i * 2})
			}
		})
	}

	// Large values (1KB)
	for _, size := range sizes {
		b.Run(fmt.Sprintf("LargeValue/size-%d", size), func(b *testing.B) {
			m := NewMap[int, LargeValue](nil)
			for i := 0; i < size; i++ {
				m = m.Set(i, LargeValue{
					ID:          i,
					Name:        fmt.Sprintf("Item-%d", i),
					Description: fmt.Sprintf("Description for item %d", i),
				})
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Set(i%size, LargeValue{
					ID:          i * 2,
					Name:        fmt.Sprintf("Updated-%d", i),
					Description: fmt.Sprintf("Updated description for %d", i),
				})
			}
		})
	}

	// Huge values (10KB)
	for _, size := range []int{100, 1000} { // Smaller sizes for huge values
		b.Run(fmt.Sprintf("HugeValue/size-%d", size), func(b *testing.B) {
			m := NewMap[int, HugeValue](nil)
			for i := 0; i < size; i++ {
				m = m.Set(i, HugeValue{
					ID:          i,
					Name:        fmt.Sprintf("Item-%d", i),
					Description: fmt.Sprintf("Description for item %d", i),
				})
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Set(i%size, HugeValue{
					ID:          i * 2,
					Name:        fmt.Sprintf("Updated-%d", i),
					Description: fmt.Sprintf("Updated description for %d", i),
				})
			}
		})
	}
}

// Comparison with Go built-in map for large values
func BenchmarkGoMapLargeValues(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	// Large values with Go map
	for _, size := range sizes {
		b.Run(fmt.Sprintf("LargeValue/size-%d", size), func(b *testing.B) {
			m := make(map[int]LargeValue, size)
			for i := 0; i < size; i++ {
				m[i] = LargeValue{
					ID:          i,
					Name:        fmt.Sprintf("Item-%d", i),
					Description: fmt.Sprintf("Description for item %d", i),
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m[i%size] = LargeValue{
					ID:          i * 2,
					Name:        fmt.Sprintf("Updated-%d", i),
					Description: fmt.Sprintf("Updated description for %d", i),
				}
			}
		})
	}
}
