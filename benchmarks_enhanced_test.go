package immutable

import (
	"fmt"
	"math/rand"
	"testing"
)

var benchmarkSizes = []int{100, 1000, 10000, 100000}

// ============================================================================
//
//                                  LIST
//
// ============================================================================

func BenchmarkList_Get(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			l := NewList[int]()
			for i := 0; i < size; i++ {
				l = l.Append(i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = l.Get(i % size)
			}
		})
	}
}

func BenchmarkSlice_Get(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			s := make([]int, 0, size)
			for i := 0; i < size; i++ {
				s = append(s, i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = s[i%size]
			}
		})
	}
}

func BenchmarkList_RandomSet(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			rng := rand.New(rand.NewSource(int64(size)))
			l := NewList[int]()
			for i := 0; i < size; i++ {
				l = l.Append(i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				l = l.Set(rng.Intn(size), i)
			}
		})
	}
}

// ============================================================================
//
//                                  MAP
//
// ============================================================================

func BenchmarkMap_Get(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			m := NewMap[int, int](nil)
			for i := 0; i < size; i++ {
				m = m.Set(i, i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = m.Get(i % size)
			}
		})
	}
}

func BenchmarkGoMap_Get(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			m := make(map[int]int, size)
			for i := 0; i < size; i++ {
				m[i] = i
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = m[i%size]
			}
		})
	}
}

func BenchmarkMap_RandomSet(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			rng := rand.New(rand.NewSource(int64(size)))
			m := NewMap[int, int](nil)
			keys := make([]int, size)
			for i := 0; i < size; i++ {
				keys[i] = rng.Int()
				m = m.Set(keys[i], i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Set(keys[rng.Intn(size)], i)
			}
		})
	}
}

func BenchmarkMap_RandomDelete(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			rng := rand.New(rand.NewSource(int64(size)))
			m := NewMap[int, int](nil)
			keys := make([]int, size)
			for i := 0; i < size; i++ {
				keys[i] = rng.Int()
				m = m.Set(keys[i], i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Delete(keys[i%size])
			}
		})
	}
}

// ============================================================================
//
//                              SORTED MAP
//
// ============================================================================

func BenchmarkSortedMap_Get(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			m := NewSortedMap[int, int](nil)
			for i := 0; i < size; i++ {
				m = m.Set(i, i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = m.Get(i % size)
			}
		})
	}
}

func BenchmarkSortedMap_RandomSet(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			rng := rand.New(rand.NewSource(int64(size)))
			m := NewSortedMap[int, int](nil)
			keys := make([]int, size)
			for i := 0; i < size; i++ {
				keys[i] = rng.Int()
				m = m.Set(keys[i], i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Set(keys[rng.Intn(size)], i)
			}
		})
	}
}

func BenchmarkSortedMap_RandomDelete(b *testing.B) {
	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			rng := rand.New(rand.NewSource(int64(size)))
			m := NewSortedMap[int, int](nil)
			keys := make([]int, size)
			for i := 0; i < size; i++ {
				keys[i] = rng.Int()
				m = m.Set(keys[i], i)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				m = m.Delete(keys[i%size])
			}
		})
	}
}
