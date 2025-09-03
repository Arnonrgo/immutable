package immutable

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
)

// BenchmarkScaling tests performance across different data sizes
func BenchmarkScaling(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("List_Append_N%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				l := NewList[int]()
				for j := 0; j < size; j++ {
					l = l.Append(j)
				}
			}
		})

		b.Run(fmt.Sprintf("ListBuilder_Append_N%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				builder := NewListBuilder[int]()
				for j := 0; j < size; j++ {
					builder.Append(j)
				}
				_ = builder.List()
			}
		})

		b.Run(fmt.Sprintf("Map_Set_N%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				m := NewMap[int, int](nil)
				for j := 0; j < size; j++ {
					m = m.Set(j, j*2)
				}
			}
		})

		b.Run(fmt.Sprintf("MapBuilder_Set_N%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				builder := NewMapBuilder[int, int](nil)
				for j := 0; j < size; j++ {
					builder.Set(j, j*2)
				}
				_ = builder.Map()
			}
		})

		b.Run(fmt.Sprintf("SortedMap_Set_N%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				m := NewSortedMap[int, int](nil)
				for j := 0; j < size; j++ {
					m = m.Set(j, j*2)
				}
			}
		})
	}
}

// BenchmarkSyncMap provides baseline numbers for Go's sync.Map
func BenchmarkSyncMap(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Store_N%d", size), func(b *testing.B) {
			var m sync.Map
			for i := 0; i < size; i++ {
				m.Store(i, i*2)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				m.Store(i%size, i)
			}
		})

		b.Run(fmt.Sprintf("Load_N%d", size), func(b *testing.B) {
			var m sync.Map
			for i := 0; i < size; i++ {
				m.Store(i, i*2)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = m.Load(i % size)
			}
		})

		b.Run(fmt.Sprintf("LoadOrStore_N%d", size), func(b *testing.B) {
			var m sync.Map
			for i := 0; i < size; i++ {
				m.Store(i, i*2)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = m.LoadOrStore(i%size, i)
			}
		})
	}
}

// BenchmarkMemoryUsage focuses on memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("List_MemoryGrowth", func(b *testing.B) {
		sizes := []int{10, 100, 1000, 10000}
		for _, size := range sizes {
			b.Run(fmt.Sprintf("N%d", size), func(b *testing.B) {
				b.ReportAllocs()
				runtime.GC()

				var m1, m2 runtime.MemStats
				runtime.ReadMemStats(&m1)

				for i := 0; i < b.N; i++ {
					l := NewList[int]()
					for j := 0; j < size; j++ {
						l = l.Append(j)
					}
					runtime.KeepAlive(l)
				}

				runtime.ReadMemStats(&m2)
				b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N)/float64(size), "bytes/element")
			})
		}
	})

	b.Run("StructuralSharing_List", func(b *testing.B) {
		b.ReportAllocs()

		// Create a base list
		base := NewList[int]()
		for i := 0; i < 1000; i++ {
			base = base.Append(i)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Creating variations should share structure
			v1 := base.Append(9999)
			v2 := base.Prepend(-1)
			v3 := base.Set(500, 12345)
			runtime.KeepAlive(v1)
			runtime.KeepAlive(v2)
			runtime.KeepAlive(v3)
		}
	})

	b.Run("StructuralSharing_Map", func(b *testing.B) {
		b.ReportAllocs()

		// Create a base map
		base := NewMap[int, int](nil)
		for i := 0; i < 1000; i++ {
			base = base.Set(i, i*2)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Creating variations should share structure
			v1 := base.Set(9999, 19998)
			v2 := base.Set(500, 12345)
			v3 := base.Delete(250)
			runtime.KeepAlive(v1)
			runtime.KeepAlive(v2)
			runtime.KeepAlive(v3)
		}
	})
}

// BenchmarkComparison compares with Go built-in types
func BenchmarkComparison(b *testing.B) {
	const size = 10000

	b.Run("SliceVsList_Sequential", func(b *testing.B) {
		b.Run("GoSlice_Append", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				slice := make([]int, 0, size)
				for j := 0; j < size; j++ {
					slice = append(slice, j)
				}
				runtime.KeepAlive(slice)
			}
		})

		b.Run("ImmutableList_Append", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				list := NewList[int]()
				for j := 0; j < size; j++ {
					list = list.Append(j)
				}
				runtime.KeepAlive(list)
			}
		})
	})

	b.Run("MapVsBuiltin_Sequential", func(b *testing.B) {
		b.Run("GoMap_Set", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				m := make(map[int]int, size)
				for j := 0; j < size; j++ {
					m[j] = j * 2
				}
				runtime.KeepAlive(m)
			}
		})

		b.Run("ImmutableMap_Set", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				m := NewMap[int, int](nil)
				for j := 0; j < size; j++ {
					m = m.Set(j, j*2)
				}
				runtime.KeepAlive(m)
			}
		})
	})
}

// BenchmarkRealWorldPatterns tests common usage scenarios
func BenchmarkRealWorldPatterns(b *testing.B) {
	b.Run("List_RandomAccess", func(b *testing.B) {
		// Build a list first
		list := NewList[int]()
		for i := 0; i < 10000; i++ {
			list = list.Append(i)
		}

		// Generate random indices
		indices := make([]int, 1000)
		for i := range indices {
			indices[i] = rand.Intn(list.Len())
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for _, idx := range indices {
				_ = list.Get(idx)
			}
		}
	})

	b.Run("Map_RandomAccess", func(b *testing.B) {
		// Build a map first
		m := NewMap[int, int](nil)
		keys := make([]int, 10000)
		for i := 0; i < 10000; i++ {
			keys[i] = i
			m = m.Set(i, i*2)
		}

		// Shuffle keys for random access
		rand.Shuffle(len(keys), func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
		})

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for j := 0; j < 1000; j++ {
				_, _ = m.Get(keys[j])
			}
		}
	})

	b.Run("List_MixedOperations", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			list := NewList[int]()

			// Build phase
			for j := 0; j < 1000; j++ {
				list = list.Append(j)
			}

			// Mixed operations
			for j := 0; j < 100; j++ {
				list = list.Prepend(-j)
				if j%2 == 0 {
					list = list.Set(j*10, j*100)
				}
				sublist := list.Slice(j, j+50)
				runtime.KeepAlive(sublist)
			}

			runtime.KeepAlive(list)
		}
	})

	b.Run("Map_MixedOperations", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			m := NewMap[int, int](nil)

			// Build phase
			for j := 0; j < 1000; j++ {
				m = m.Set(j, j*2)
			}

			// Mixed operations
			for j := 0; j < 100; j++ {
				m = m.Set(j+2000, j*3)
				if j%3 == 0 {
					m = m.Delete(j)
				}
				_, exists := m.Get(j * 2)
				runtime.KeepAlive(exists)
			}

			runtime.KeepAlive(m)
		}
	})
}

// Concurrent read benchmarks: immutable Map vs sync.Map
func BenchmarkConcurrentReads(b *testing.B) {
	const size = 100000

	// immutable Map setup
	imm := NewMap[int, int](nil)
	for i := 0; i < size; i++ {
		imm = imm.Set(i, i*2)
	}

	// sync.Map setup
	var sm sync.Map
	for i := 0; i < size; i++ {
		sm.Store(i, i*2)
	}

	for _, goroutines := range []int{1, 2, 4, 8, 16} {
		b.Run(fmt.Sprintf("ImmutableMap_%dG", goroutines), func(b *testing.B) {
			b.ReportAllocs()
			b.SetParallelism(goroutines)
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_, _ = imm.Get(i % size)
					i++
				}
			})
		})

		b.Run(fmt.Sprintf("SyncMap_%dG", goroutines), func(b *testing.B) {
			b.ReportAllocs()
			b.SetParallelism(goroutines)
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_, _ = sm.Load(i % size)
					i++
				}
			})
		})
	}
}

// Mixed read/write concurrent benchmarks
func BenchmarkConcurrentMixed(b *testing.B) {
	const size = 100000
	// immutable Map setup
	base := NewMap[int, int](nil)
	for i := 0; i < size; i++ {
		base = base.Set(i, i*2)
	}
	// sync.Map setup
	var sm sync.Map
	for i := 0; i < size; i++ {
		sm.Store(i, i*2)
	}

	type mix struct{ readers, writers int }
	mixes := []mix{{9, 1}, {7, 3}, {5, 5}}

	for _, m := range mixes {
		b.Run(fmt.Sprintf("Immutable_%dR_%dW", m.readers, m.writers), func(b *testing.B) {
			b.ReportAllocs()
			var wg sync.WaitGroup
			wg.Add(m.readers + m.writers)
			stop := make(chan struct{})

			// Readers
			for r := 0; r < m.readers; r++ {
				go func() {
					defer wg.Done()
					i := 0
					for {
						select {
						case <-stop:
							return
						default:
							_, _ = base.Get(i % size)
							i++
						}
					}
				}()
			}
			// Writers: copy-on-write; advance a shadow map
			shadow := base
			var mu sync.Mutex
			for w := 0; w < m.writers; w++ {
				go func() {
					defer wg.Done()
					i := 0
					for {
						select {
						case <-stop:
							return
						default:
							mu.Lock()
							shadow = shadow.Set(i%size, i)
							mu.Unlock()
							i++
						}
					}
				}()
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = base.Get(i % size)
			}
			close(stop)
			wg.Wait()
		})

		b.Run(fmt.Sprintf("SyncMap_%dR_%dW", m.readers, m.writers), func(b *testing.B) {
			b.ReportAllocs()
			var wg sync.WaitGroup
			wg.Add(m.readers + m.writers)
			stop := make(chan struct{})

			for r := 0; r < m.readers; r++ {
				go func() {
					defer wg.Done()
					i := 0
					for {
						select {
						case <-stop:
							return
						default:
							_, _ = sm.Load(i % size)
							i++
						}
					}
				}()
			}

			for w := 0; w < m.writers; w++ {
				go func() {
					defer wg.Done()
					i := 0
					for {
						select {
						case <-stop:
							return
						default:
							sm.Store(i%size, i)
							i++
						}
					}
				}()
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = sm.Load(i % size)
			}
			close(stop)
			wg.Wait()
		})
	}
}

// Phase 4 Enhanced Builder Benchmarks

func BenchmarkBatchListBuilder(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	batchSizes := []int{16, 32, 64, 128}

	for _, size := range sizes {
		for _, batchSize := range batchSizes {
			b.Run(fmt.Sprintf("Size%d_Batch%d", size, batchSize), func(b *testing.B) {
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					builder := NewBatchListBuilder[int](batchSize)
					for j := 0; j < size; j++ {
						builder.Append(j)
					}
					_ = builder.List()
				}
			})
		}
	}
}

func BenchmarkBatchListBuilder_vs_Regular(b *testing.B) {
	const size = 10000

	b.Run("BatchBuilder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewBatchListBuilder[int](64)
			for j := 0; j < size; j++ {
				builder.Append(j)
			}
			_ = builder.List()
		}
	})

	b.Run("RegularBuilder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewListBuilder[int]()
			for j := 0; j < size; j++ {
				builder.Append(j)
			}
			_ = builder.List()
		}
	})

	b.Run("DirectConstruction", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			list := NewList[int]()
			for j := 0; j < size; j++ {
				list = list.Append(j)
			}
		}
	})
}

func BenchmarkBatchMapBuilder(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	batchSizes := []int{16, 32, 64, 128}

	for _, size := range sizes {
		for _, batchSize := range batchSizes {
			b.Run(fmt.Sprintf("Size%d_Batch%d", size, batchSize), func(b *testing.B) {
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					builder := NewBatchMapBuilder[int, string](nil, batchSize)
					for j := 0; j < size; j++ {
						builder.Set(j, fmt.Sprintf("value-%d", j))
					}
					_ = builder.Map()
				}
			})
		}
	}
}

func BenchmarkBatchMapBuilder_vs_Regular(b *testing.B) {
	const size = 10000

	b.Run("BatchBuilder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewBatchMapBuilder[int, string](nil, 64)
			for j := 0; j < size; j++ {
				builder.Set(j, fmt.Sprintf("value-%d", j))
			}
			_ = builder.Map()
		}
	})

	b.Run("RegularBuilder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewMapBuilder[int, string](nil)
			for j := 0; j < size; j++ {
				builder.Set(j, fmt.Sprintf("value-%d", j))
			}
			_ = builder.Map()
		}
	})
}

func BenchmarkStreamingListBuilder(b *testing.B) {
	const size = 10000
	const batchSize = 64
	const autoFlushSize = 1000

	b.Run("WithAutoFlush", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewStreamingListBuilder[int](batchSize, autoFlushSize)
			for j := 0; j < size; j++ {
				builder.Append(j)
			}
			_ = builder.List()
		}
	})

	b.Run("WithoutAutoFlush", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewStreamingListBuilder[int](batchSize, 0)
			for j := 0; j < size; j++ {
				builder.Append(j)
			}
			_ = builder.List()
		}
	})
}

func BenchmarkStreamingListBuilder_Operations(b *testing.B) {
	const size = 1000
	data := make([]int, size)
	for i := range data {
		data[i] = i
	}

	b.Run("Filter", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewStreamingListBuilder[int](32, 0)
			builder.Filter(data, func(x int) bool { return x%2 == 0 })
			_ = builder.List()
		}
	})

	b.Run("Transform", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewStreamingListBuilder[int](32, 0)
			builder.Transform(data, func(x int) int { return x * 2 })
			_ = builder.List()
		}
	})
}

// Small-structure Map builder benchmarks (array-node fast paths)
func BenchmarkSmallMap_BatchBuilder(b *testing.B) {
	sizes := []int{1, 2, 4, 8}

	b.Run("InitialFlush", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("N%d", size), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					builder := NewBatchMapBuilder[int, int](nil, size)
					for j := 0; j < size; j++ {
						builder.Set(j, j*2)
					}
					_ = builder.Map()
				}
			})
		}
	})

	b.Run("UpdateWithinThreshold", func(b *testing.B) {
		for _, size := range sizes {
			if size < 2 {
				continue
			}
			b.Run(fmt.Sprintf("N%d", size), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					// Start with size/2 existing entries
					builder := NewBatchMapBuilder[int, int](nil, size)
					for j := 0; j < size/2; j++ {
						builder.Set(j, j)
					}
					m := builder.Map()

					// Now flush remaining entries (updates + new) within threshold
					builder2 := NewBatchMapBuilder[int, int](nil, size)
					// attach existing map into builder2 by direct field move
					builder2.m = m
					// updates for first half
					for j := 0; j < size/2; j++ {
						builder2.Set(j, j*10)
					}
					// new keys
					for j := size / 2; j < size; j++ {
						builder2.Set(j, j*10)
					}
					_ = builder2.Map()
				}
			})
		}
	})
}

func BenchmarkSortedBatchBuilder(b *testing.B) {
	const size = 1000

	b.Run("SortedBuffer", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewSortedBatchBuilder[int, string](nil, 32, true)
			for j := 0; j < size; j++ {
				// Insert in reverse order to test sorting
				builder.Set(size-j, fmt.Sprintf("value-%d", size-j))
			}
			_ = builder.SortedMap()
		}
	})

	b.Run("UnsortedBuffer", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := NewSortedBatchBuilder[int, string](nil, 32, false)
			for j := 0; j < size; j++ {
				builder.Set(size-j, fmt.Sprintf("value-%d", size-j))
			}
			_ = builder.SortedMap()
		}
	})
}

// PGO Performance Tracking Benchmarks
// These generate profiles for PGO and measure improvements

func BenchmarkPGO_MapOperations_Heavy(b *testing.B) {
	// Heavy map operations for PGO profiling
	const size = 50000

	b.Run("RandomSet", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := NewMap[int, string](nil)
			for j := 0; j < size; j++ {
				m = m.Set(j%1000, fmt.Sprintf("value-%d", j))
			}
		}
	})

	b.Run("SequentialSet", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := NewMap[int, string](nil)
			for j := 0; j < size; j++ {
				m = m.Set(j, fmt.Sprintf("value-%d", j))
			}
		}
	})

	b.Run("MixedOperations", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m := NewMap[int, string](nil)

			// Build up map
			for j := 0; j < size/2; j++ {
				m = m.Set(j, fmt.Sprintf("value-%d", j))
			}

			// Read operations
			for j := 0; j < size/4; j++ {
				_, _ = m.Get(j)
			}

			// Delete operations
			for j := 0; j < size/8; j++ {
				m = m.Delete(j)
			}
		}
	})
}

func BenchmarkPGO_ListOperations_Heavy(b *testing.B) {
	// Heavy list operations for PGO profiling
	const size = 10000

	b.Run("AppendHeavy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			list := NewList[int]()
			for j := 0; j < size; j++ {
				list = list.Append(j)
			}
		}
	})

	b.Run("PrependHeavy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			list := NewList[int]()
			for j := 0; j < size; j++ {
				list = list.Prepend(j)
			}
		}
	})

	b.Run("RandomAccess", func(b *testing.B) {
		// Pre-build a large list
		list := NewList[int]()
		for j := 0; j < size; j++ {
			list = list.Append(j)
		}

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 1000; j++ {
				_ = list.Get(j % size)
			}
		}
	})
}

func BenchmarkPGO_SortedMapOperations_Heavy(b *testing.B) {
	// Heavy sorted map operations for PGO profiling
	const size = 20000

	b.Run("SortedInsertion", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sm := NewSortedMap[int, string](nil)
			for j := 0; j < size; j++ {
				sm = sm.Set(j, fmt.Sprintf("value-%d", j))
			}
		}
	})

	b.Run("ReverseSortedInsertion", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sm := NewSortedMap[int, string](nil)
			for j := size; j > 0; j-- {
				sm = sm.Set(j, fmt.Sprintf("value-%d", j))
			}
		}
	})
}
