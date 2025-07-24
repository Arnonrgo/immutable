package immutable

import (
	"cmp"
	"testing"
)

// Enhanced benchmarks using Go 1.24+ testing.B.Loop
func BenchmarkEnhanced_MapSet_Loop(b *testing.B) {
	m := NewMap[int, string](nil)

	for b.Loop() {
		m = m.Set(b.N, "value")
	}
}

func BenchmarkEnhanced_ListAppend_Loop(b *testing.B) {
	l := NewList[int]()

	for b.Loop() {
		l = l.Append(b.N)
	}
}

// Benchmark using built-in min/max vs custom functions
func BenchmarkBuiltin_MinMax(b *testing.B) {
	values := []int{1, 5, 3, 9, 2, 7, 4, 8, 6}

	b.Run("BuiltIn", func(b *testing.B) {
		for b.Loop() {
			_ = min(values[0], values[1], values[2])
			_ = max(values[3], values[4], values[5])
		}
	})

	b.Run("Custom", func(b *testing.B) {
		for b.Loop() {
			_ = customMin(values[0], values[1], values[2])
			_ = customMax(values[3], values[4], values[5])
		}
	})
}

// Custom implementations for comparison
func customMin[T cmp.Ordered](a, b, c T) T {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func customMax[T cmp.Ordered](a, b, c T) T {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

// Benchmark comparing cmp.Compare vs custom comparison
func BenchmarkCmp_Compare(b *testing.B) {
	values := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	b.Run("CmpCompare", func(b *testing.B) {
		for b.Loop() {
			for i := 0; i < len(values)-1; i++ {
				_ = cmp.Compare(values[i], values[i+1])
			}
		}
	})

	b.Run("CustomCompare", func(b *testing.B) {
		for b.Loop() {
			for i := 0; i < len(values)-1; i++ {
				_ = defaultCompare(values[i], values[i+1])
			}
		}
	})
}

// Benchmark built-in clear vs manual clearing
func BenchmarkBuiltin_Clear(b *testing.B) {
	b.Run("BuiltInClear", func(b *testing.B) {
		for b.Loop() {
			slice := make([]int, 1000)
			for i := range slice {
				slice[i] = i
			}
			clear(slice)
		}
	})

	b.Run("ManualClear", func(b *testing.B) {
		for b.Loop() {
			slice := make([]int, 1000)
			for i := range slice {
				slice[i] = i
			}
			for i := range slice {
				slice[i] = 0
			}
		}
	})
}
