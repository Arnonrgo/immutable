package immutable

import (
	"fmt"
	"testing"
)

// largeValueHasher provides a lightweight Hasher for LargeValue so Sets can use it as a key type.
type largeValueHasher struct{}

func (h *largeValueHasher) Hash(v LargeValue) uint32 {
	// Fast, stable hash combining ID and Name; adequate for benchmarking.
	hash := uint32(v.ID)
	for i := 0; i < len(v.Name); i++ {
		hash = 31*hash + uint32(v.Name[i])
	}
	return hash
}

func (h *largeValueHasher) Equal(a, b LargeValue) bool {
	// Consider values equal if IDs match for set semantics during benchmarks.
	return a.ID == b.ID
}

// Benchmark Set operations to show they inherit Map optimizations
func BenchmarkSet_Operations(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Add/size-%d", size), func(b *testing.B) {
			s := NewSet[int](nil)
			for i := 0; i < size; i++ {
				s = s.Add(i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				s = s.Add(i % size) // Re-adding existing values
			}
		})

		b.Run(fmt.Sprintf("Delete/size-%d", size), func(b *testing.B) {
			s := NewSet[int](nil)
			for i := 0; i < size; i++ {
				s = s.Add(i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Persistent delete of an existing value from the original, full-size
			// set. The result is discarded so the set never shrinks; feeding it
			// back would turn every Delete after the first size iterations into a
			// no-op on a missing value.
			for i := 0; i < b.N; i++ {
				sinkAny = s.Delete(i % size)
			}
		})

		b.Run(fmt.Sprintf("Has/size-%d", size), func(b *testing.B) {
			s := NewSet[int](nil)
			for i := 0; i < size; i++ {
				s = s.Add(i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				sinkBool = s.Has(i % size)
			}
		})
	}
}

// Benchmark SortedSet operations
func BenchmarkSortedSet_Operations(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Add/size-%d", size), func(b *testing.B) {
			s := NewSortedSet[int](nil)
			for i := 0; i < size; i++ {
				s = s.Add(i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				s = s.Add(i % size)
			}
		})

		b.Run(fmt.Sprintf("Has/size-%d", size), func(b *testing.B) {
			s := NewSortedSet[int](nil)
			for i := 0; i < size; i++ {
				s = s.Add(i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = s.Has(i % size)
			}
		})
	}
}

// Benchmark Set with large values to show scaling benefits
func BenchmarkSet_LargeValues(b *testing.B) {
	sizes := []int{100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("LargeStruct/size-%d", size), func(b *testing.B) {
			s := NewSet[LargeValue](&largeValueHasher{}) // Reuse our 1KB struct
			for i := 0; i < size; i++ {
				s = s.Add(LargeValue{
					ID:          i,
					Name:        fmt.Sprintf("Item-%d", i),
					Description: fmt.Sprintf("Description for item %d", i),
				})
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				s = s.Add(LargeValue{
					ID:          i % size,
					Name:        fmt.Sprintf("Updated-%d", i),
					Description: fmt.Sprintf("Updated description for %d", i),
				})
			}
		})
	}
}

// Benchmark Set builders
func BenchmarkSetBuilder(b *testing.B) {
	b.Run("SetBuilder", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			builder := NewSetBuilder[int](nil)
			for j := 0; j < 1000; j++ {
				builder.Add(j)
			}
			_ = builder.Set()
		}
	})

	b.Run("SortedSetBuilder", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			builder := NewSortedSetBuilder[int](nil)
			for j := 0; j < 1000; j++ {
				builder.Add(j)
			}
			_ = builder.SortedSet() // Get final set
		}
	})
}

// Compare with Go's built-in map[T]bool pattern
func BenchmarkGoMapAsSet(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Add/size-%d", size), func(b *testing.B) {
			m := make(map[int]bool, size)
			for i := 0; i < size; i++ {
				m[i] = true
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m[i%size] = true
			}
		})

		b.Run(fmt.Sprintf("Has/size-%d", size), func(b *testing.B) {
			m := make(map[int]bool, size)
			for i := 0; i < size; i++ {
				m[i] = true
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = m[i%size]
			}
		})
	}
}
