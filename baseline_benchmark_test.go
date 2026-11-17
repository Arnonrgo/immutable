package immutable

import (
	"fmt"
	"testing"
)

// Baseline benchmarks: the curated set used as ground truth for the
// efficiency work (memory-bank/plans/efficiency-and-review-followups.md).
//
// Conventions:
//   - Names start with BenchmarkBaseline_ so scripts/bench.sh can select them.
//   - Benchmarks named ".../N<size>" build a whole collection of <size>
//     elements per iteration and report an "ns/elem" metric. Use B/op and
//     allocs/op divided by <size> for per-element memory.
//   - Benchmarks that measure a single operation (Get, RandomSet) run on a
//     pre-built, full-size collection and never feed their result back.
//   - Inputs are fixed (seeded) and generated before the timed region.
//   - Results go to package sinks so nothing is optimized away.

// ----------------------------------------------------------------------------
// List
// ----------------------------------------------------------------------------

// BenchmarkBaseline_List_Append builds a list with the persistent Append loop.
func BenchmarkBaseline_List_Append(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				l := NewList[int]()
				for j := 0; j < n; j++ {
					l = l.Append(j)
				}
				sinkAny = l
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_List_Builder builds a list with ListBuilder.
func BenchmarkBaseline_List_Builder(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				builder := NewListBuilder[int]()
				for j := 0; j < n; j++ {
					builder.Append(j)
				}
				sinkAny = builder.List()
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_List_BatchBuilder builds a list with BatchListBuilder at
// several batch sizes.
func BenchmarkBaseline_List_BatchBuilder(b *testing.B) {
	for _, n := range benchBaselineSizes {
		for _, batch := range []int{16, 128, 1024} {
			b.Run(fmt.Sprintf("N%d/batch%d", n, batch), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					builder := NewBatchListBuilder[int](batch)
					for j := 0; j < n; j++ {
						builder.Append(j)
					}
					sinkAny = builder.List()
				}
				reportPerElem(b, n)
			})
		}
	}
}

// BenchmarkBaseline_List_New builds a list from an existing slice with NewList.
func BenchmarkBaseline_List_New(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			values := benchIntKeys(n, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkAny = NewList(values...)
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_List_Set measures one persistent Set at a random index of a
// full-size list. The result is discarded, so the list never changes.
func BenchmarkBaseline_List_Set(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			l := NewList(benchIntKeys(n, false)...)
			idx := benchIntKeys(n, true)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkAny = l.Set(int(uint(idx[i%n])%uint(n)), i)
			}
		})
	}
}

// BenchmarkBaseline_List_Get measures one Get on a full-size list.
func BenchmarkBaseline_List_Get(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			l := NewList(benchIntKeys(n, false)...)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkInt = l.Get(i % n)
			}
		})
	}
}

// BenchmarkBaseline_List_Contains searches for a value that is not in the list,
// so every element is compared. N16 is slice-backed; the larger sizes are
// trie-backed (built with the persistent Append loop).
func BenchmarkBaseline_List_Contains(b *testing.B) {
	for _, n := range []int{16, 1000, 100000} {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			l := NewList[int]()
			for j := 0; j < n; j++ {
				l = l.Append(j)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkBool = l.Contains(-1)
			}
			reportPerElem(b, n)
		})
	}
}

// ----------------------------------------------------------------------------
// Map (int keys): sequential keys vs random keys show hash quality.
// ----------------------------------------------------------------------------

// BenchmarkBaseline_Map_Set builds a map with the persistent Set loop.
func BenchmarkBaseline_Map_Set(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					m := NewMap[int, int](nil)
					for j, k := range keys {
						m = m.Set(k, j)
					}
					sinkAny = m
				}
				reportPerElem(b, n)
			})
		}
	}
}

// BenchmarkBaseline_Map_Builder builds a map with MapBuilder.
func BenchmarkBaseline_Map_Builder(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					builder := NewMapBuilder[int, int](nil)
					for j, k := range keys {
						builder.Set(k, j)
					}
					sinkAny = builder.Map()
				}
				reportPerElem(b, n)
			})
		}
	}
}

// BenchmarkBaseline_Map_BatchBuilder builds a map with BatchMapBuilder at
// several batch sizes, with random keys.
func BenchmarkBaseline_Map_BatchBuilder(b *testing.B) {
	for _, n := range benchBaselineSizes {
		for _, batch := range []int{16, 128, 1024} {
			b.Run(fmt.Sprintf("rand/N%d/batch%d", n, batch), func(b *testing.B) {
				keys := benchIntKeys(n, true)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					builder := NewBatchMapBuilder[int, int](nil, batch)
					for j, k := range keys {
						builder.Set(k, j)
					}
					sinkAny = builder.Map()
				}
				reportPerElem(b, n)
			})
		}
	}
}

// BenchmarkBaseline_Map_Get measures one Get on a full-size map.
func BenchmarkBaseline_Map_Get(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				builder := NewMapBuilder[int, int](nil)
				for j, k := range keys {
					builder.Set(k, j)
				}
				m := builder.Map()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sinkInt, sinkBool = m.Get(keys[i%n])
				}
			})
		}
	}
}

// BenchmarkBaseline_Map_SetOne measures one persistent Set of a new key into a
// full-size map. The result is discarded, so the map never grows.
func BenchmarkBaseline_Map_SetOne(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(2*n, pattern == "rand")
				builder := NewMapBuilder[int, int](nil)
				for j := 0; j < n; j++ {
					builder.Set(keys[j], j)
				}
				m := builder.Map()
				extra := keys[n:] // keys not in the map
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sinkAny = m.Set(extra[i%n], i)
				}
			})
		}
	}
}

// BenchmarkBaseline_Map_DeleteOne measures one persistent Delete of an existing
// key from a full-size map. The result is discarded, so the map never shrinks.
func BenchmarkBaseline_Map_DeleteOne(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				builder := NewMapBuilder[int, int](nil)
				for j, k := range keys {
					builder.Set(k, j)
				}
				m := builder.Map()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sinkAny = m.Delete(keys[i%n])
				}
			})
		}
	}
}

// BenchmarkBaseline_GoMap_Set builds a built-in Go map: the reference point.
func BenchmarkBaseline_GoMap_Set(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					m := make(map[int]int)
					for j, k := range keys {
						m[k] = j
					}
					sinkAny = m
				}
				reportPerElem(b, n)
			})
		}
	}
}

// ----------------------------------------------------------------------------
// Map (string keys)
// ----------------------------------------------------------------------------

// BenchmarkBaseline_MapString_Set builds a string-keyed map with the
// persistent Set loop.
func BenchmarkBaseline_MapString_Set(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			keys := benchStringKeys(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := NewMap[string, int](nil)
				for j, k := range keys {
					m = m.Set(k, j)
				}
				sinkAny = m
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_MapString_Builder builds a string-keyed map with MapBuilder.
func BenchmarkBaseline_MapString_Builder(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			keys := benchStringKeys(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder := NewMapBuilder[string, int](nil)
				for j, k := range keys {
					builder.Set(k, j)
				}
				sinkAny = builder.Map()
			}
			reportPerElem(b, n)
		})
	}
}

// BenchmarkBaseline_MapString_Get measures one Get on a full-size string-keyed map.
func BenchmarkBaseline_MapString_Get(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			keys := benchStringKeys(n)
			builder := NewMapBuilder[string, int](nil)
			for j, k := range keys {
				builder.Set(k, j)
			}
			m := builder.Map()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkInt, sinkBool = m.Get(keys[i%n])
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Sorted map and set
// ----------------------------------------------------------------------------

// BenchmarkBaseline_SortedMap_Builder builds a sorted map with SortedMapBuilder.
func BenchmarkBaseline_SortedMap_Builder(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					builder := NewSortedMapBuilder[int, int](nil)
					for j, k := range keys {
						builder.Set(k, j)
					}
					sinkAny = builder.Map()
				}
				reportPerElem(b, n)
			})
		}
	}
}

// BenchmarkBaseline_SortedMap_BatchBuilder builds a sorted map with
// SortedBatchBuilder.
func BenchmarkBaseline_SortedMap_BatchBuilder(b *testing.B) {
	for _, pattern := range []string{"seq", "rand"} {
		for _, n := range benchBaselineSizes {
			b.Run(fmt.Sprintf("%s/N%d", pattern, n), func(b *testing.B) {
				keys := benchIntKeys(n, pattern == "rand")
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					builder := NewSortedBatchBuilder[int, int](nil, 32)
					for j, k := range keys {
						builder.Set(k, j)
					}
					sinkAny = builder.SortedMap()
				}
				reportPerElem(b, n)
			})
		}
	}
}

// BenchmarkBaseline_SortedMap_Get measures one Get on a full-size sorted map.
func BenchmarkBaseline_SortedMap_Get(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("rand/N%d", n), func(b *testing.B) {
			keys := benchIntKeys(n, true)
			builder := NewSortedMapBuilder[int, int](nil)
			for j, k := range keys {
				builder.Set(k, j)
			}
			m := builder.Map()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkInt, sinkBool = m.Get(keys[i%n])
			}
		})
	}
}

// BenchmarkBaseline_Set_Builder builds a set with SetBuilder.
func BenchmarkBaseline_Set_Builder(b *testing.B) {
	for _, n := range benchBaselineSizes {
		b.Run(fmt.Sprintf("rand/N%d", n), func(b *testing.B) {
			keys := benchIntKeys(n, true)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder := NewSetBuilder[int](nil)
				for _, k := range keys {
					builder.Add(k)
				}
				sinkAny = builder.Set()
			}
			reportPerElem(b, n)
		})
	}
}
