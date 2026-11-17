package immutable

import (
	"math/rand"
	"strconv"
	"testing"
)

// Package-level sinks keep benchmark results observable, so the compiler
// cannot eliminate the work being measured (dead-code elimination or inlining
// of a call whose result is discarded). Assign results to a sink instead of
// "_" or an unused local.
var (
	sinkInt  int
	sinkBool bool
	sinkAny  any
)

// benchBaselineSizes are the collection sizes used by the baseline benchmarks.
var benchBaselineSizes = []int{1000, 10000, 100000}

// reportPerElem adds an "ns/elem" metric to a benchmark whose single operation
// builds a collection of n elements, so results at different sizes compare.
func reportPerElem(b *testing.B, n int) {
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/elem")
}

// benchIntKeys returns n distinct int keys. If random is false the keys are
// sequential (0, 1, 2, ...); otherwise they are distinct pseudo-random ints
// from a fixed seed, so runs are reproducible.
func benchIntKeys(n int, random bool) []int {
	keys := make([]int, n)
	if !random {
		for i := range keys {
			keys[i] = i
		}
		return keys
	}
	rng := rand.New(rand.NewSource(int64(n)))
	seen := make(map[int]struct{}, n)
	for i := range keys {
		for {
			k := rng.Int()
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				keys[i] = k
				break
			}
		}
	}
	return keys
}

// benchStringKeys returns n distinct string keys ("key-0", "key-1", ...).
// Call it outside the timed region.
func benchStringKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}
	return keys
}
