package immutable

import (
	"fmt"
	"math/rand"
	"testing"
)

var benchmarkSizes = []int{100, 1000, 10000, 100000}

// Notes on benchmark methodology (see memory-bank/plans/efficiency-and-review-followups.md, Phase 0):
//   - Results are assigned to package-level sinks so the compiler cannot elide the work.
//   - Persistent Delete/Set benchmarks always operate on the original, full-size
//     collection and discard the result. This keeps every iteration a real
//     operation at a constant size. (Feeding the result back would shrink the
//     collection until Delete becomes a no-op on a missing key.)
//   - Random keys and indexes are generated before the timed region.

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
				sinkInt = l.Get(i % size)
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
				sinkInt = s[i%size]
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
			idx := make([]int, size)
			for i := range idx {
				idx[i] = rng.Intn(size)
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				l = l.Set(idx[i%size], i)
			}
			sinkAny = l
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
				sinkInt, sinkBool = m.Get(i % size)
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
				sinkInt, sinkBool = m[i%size]
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

			// keys is already in random order, so cycling through it gives a
			// random access pattern without timing the RNG.
			for i := 0; i < b.N; i++ {
				m = m.Set(keys[i%size], i)
			}
			sinkAny = m
		})
	}
}

// BenchmarkMap_RandomDelete measures a persistent Delete of an existing key
// from a full-size map. The result is discarded, so the map never shrinks.
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
				sinkAny = m.Delete(keys[i%size])
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
				sinkInt, sinkBool = m.Get(i % size)
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
				m = m.Set(keys[i%size], i)
			}
			sinkAny = m
		})
	}
}

// BenchmarkSortedMap_RandomDelete measures a persistent Delete of an existing
// key from a full-size map. The result is discarded, so the map never shrinks.
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
				sinkAny = m.Delete(keys[i%size])
			}
		})
	}
}
