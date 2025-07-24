# Baseline Performance Metrics
*Recorded: 2024-12-19*  
*Go Version: 1.18+*  
*Platform: Apple M2 Pro (arm64)*  
*Test Environment: macOS Darwin*

## 🚀 **Benchmark Results (Before Optimization)**

### **List Performance**
```
BenchmarkList_Get/size-100-12           214218867    7.283 ns/op    0 B/op    0 allocs/op
BenchmarkList_Get/size-1000-12          194329256    6.002 ns/op    0 B/op    0 allocs/op
BenchmarkList_Get/size-10000-12         167195188    5.995 ns/op    0 B/op    0 allocs/op
BenchmarkList_Get/size-100000-12        146882816    8.096 ns/op    0 B/op    0 allocs/op

BenchmarkList_RandomSet/size-100-12      4901679     254.7 ns/op   1472 B/op   4 allocs/op
BenchmarkList_RandomSet/size-1000-12     4713792     269.3 ns/op   1472 B/op   4 allocs/op
BenchmarkList_RandomSet/size-10000-12    3408625     354.3 ns/op   2048 B/op   5 allocs/op
BenchmarkList_RandomSet/size-100000-12   2086813     607.5 ns/op   2624 B/op   6 allocs/op
```

### **Map Performance** 
```
BenchmarkMap_Get/size-100-12             94804821    12.27 ns/op    0 B/op    0 allocs/op
BenchmarkMap_Get/size-1000-12           100000000    12.36 ns/op    0 B/op    0 allocs/op
BenchmarkMap_Get/size-10000-12           82940947    14.83 ns/op    0 B/op    0 allocs/op
BenchmarkMap_Get/size-100000-12          66408105    18.46 ns/op    0 B/op    0 allocs/op

BenchmarkMap_RandomSet/size-100-12        4683004    261.5 ns/op    745 B/op   6 allocs/op
BenchmarkMap_RandomSet/size-1000-12       3083697    422.2 ns/op   1222 B/op   6 allocs/op
BenchmarkMap_RandomSet/size-10000-12      1798087    693.2 ns/op   1421 B/op   7 allocs/op
BenchmarkMap_RandomSet/size-100000-12     1000000   1213 ns/op     1897 B/op   8 allocs/op

BenchmarkMap_RandomDelete/size-100-12    520608222    2.490 ns/op    0 B/op    0 allocs/op
BenchmarkMap_RandomDelete/size-1000-12   516804304    2.318 ns/op    0 B/op    0 allocs/op
BenchmarkMap_RandomDelete/size-10000-12  512492904    2.334 ns/op    0 B/op    0 allocs/op
BenchmarkMap_RandomDelete/size-100000-12 394076763    3.000 ns/op    0 B/op    0 allocs/op
```

### **SortedMap Performance**
```
BenchmarkSortedMap_Get/size-100-12        24631194    45.69 ns/op    0 B/op    0 allocs/op
BenchmarkSortedMap_Get/size-1000-12       11693418    85.72 ns/op    0 B/op    0 allocs/op
BenchmarkSortedMap_Get/size-10000-12      13150275    94.66 ns/op    0 B/op    0 allocs/op
BenchmarkSortedMap_Get/size-100000-12     12092004   102.0 ns/op     0 B/op    0 allocs/op

BenchmarkSortedMap_RandomSet/size-100-12   4298673    310.2 ns/op    625 B/op   6 allocs/op
BenchmarkSortedMap_RandomSet/size-1000-12  2058636    526.8 ns/op   1118 B/op   8 allocs/op
BenchmarkSortedMap_RandomSet/size-10000-12 1278103    881.9 ns/op   1601 B/op   8 allocs/op
BenchmarkSortedMap_RandomSet/size-100000-12 863854   1535 ns/op     1955 B/op  10 allocs/op

BenchmarkSortedMap_RandomDelete/size-100-12    483693970    2.407 ns/op    0 B/op    0 allocs/op
BenchmarkSortedMap_RandomDelete/size-1000-12   513868102    2.581 ns/op    0 B/op    0 allocs/op
BenchmarkSortedMap_RandomDelete/size-10000-12  478816365    2.428 ns/op    0 B/op    0 allocs/op
BenchmarkSortedMap_RandomDelete/size-100000-12 396879606    2.772 ns/op    0 B/op    0 allocs/op
```

### **Comparison to Go Built-ins**
```
BenchmarkSlice_Get/size-100-12       1000000000    0.6057 ns/op    0 B/op    0 allocs/op
BenchmarkSlice_Get/size-1000-12      1000000000    0.6110 ns/op    0 B/op    0 allocs/op
BenchmarkSlice_Get/size-10000-12     1000000000    0.6106 ns/op    0 B/op    0 allocs/op
BenchmarkSlice_Get/size-100000-12    1000000000    0.6088 ns/op    0 B/op    0 allocs/op

BenchmarkGoMap_Get/size-100-12       216990180     5.525 ns/op     0 B/op    0 allocs/op
BenchmarkGoMap_Get/size-1000-12      220556931     5.828 ns/op     0 B/op    0 allocs/op
BenchmarkGoMap_Get/size-10000-12     163907499     7.232 ns/op     0 B/op    0 allocs/op
BenchmarkGoMap_Get/size-100000-12    100000000    10.22 ns/op      0 B/op    0 allocs/op
```

## 📊 **Memory Profiling Data (Before Optimization)**

### **Memory Allocation Hotspots:**
```
Showing nodes accounting for 116689.43MB, 98.30% of 118701.79MB total

63287.75MB (53.32%) - mapHashArrayNode.clone ⭐ PRIMARY TARGET
25385.94MB (21.39%) - listBranchNode.set
 6939.69MB  (5.85%) - mapBitmapIndexedNode.set  
 5533.98MB  (4.66%) - sortedMapBranchNode.set
 4768.81MB  (4.02%) - listLeafNode.set
 2532.62MB  (2.13%) - Map.clone
 1926.12MB  (1.62%) - List.Prepend
 1766.67MB  (1.49%) - sortedMapLeafNode.set
 1275.03MB  (1.07%) - newMapValueNode
 1203.33MB  (1.01%) - sortedMapBranchNode.delete
```

### **CPU Profiling Data:**
```
Total CPU Time: 109.17s
Duration: 41.37s (263.87% CPU usage)

Application Functions:
5.70s (5.22%) - Map.set
4.77s (4.37%) - mapHashArrayNode.set  
3.57s (3.27%) - listBranchNode.set
1.51s (1.38%) - mapBitmapIndexedNode.set
```

## 🎯 **Key Performance Characteristics**

### **Read Operations (Excellent - Zero Allocations)**
- **List Get**: 6-8 ns/op (vs slice 0.6ns) = **~10x overhead**
- **Map Get**: 12-18 ns/op (vs Go map 5-10ns) = **~2x overhead**  
- **SortedMap Get**: 45-102 ns/op = **~4-10x slower than Map**

### **Write Operations (High Allocation Cost)**
- **List Set**: 254-607 ns/op, 4-6 allocations, 1.4-2.6KB per op
- **Map Set**: 261-1213 ns/op, 6-8 allocations, 0.7-1.9KB per op
- **SortedMap Set**: 310-1535 ns/op, 6-10 allocations, 0.6-2.0KB per op

### **Delete Operations (Excellent - Zero Allocations)**
- **Map Delete**: 2-3 ns/op, 0 allocations ⚡
- **SortedMap Delete**: 2-3 ns/op, 0 allocations ⚡

### **Scaling Behavior**
- **Read operations**: Scale very well, minimal degradation with size
- **Write operations**: Performance decreases roughly 2-4x from 100 to 100K elements
- **Memory per operation**: Increases with collection size (more tree depth)

## 🔍 **Identified Performance Issues**

### **Critical Bottlenecks:**
1. **mapHashArrayNode.clone()** consuming 53% of all memory allocations
2. **listBranchNode.set()** consuming 21% of all memory allocations  
3. Excessive GC pressure from structural copying
4. Full array copying for single element changes

### **Architecture Issues:**
- Fixed-size arrays `[32]mapNode[K,V]` copied entirely on each modification
- Recursive tree copying cascades memory allocations
- No lazy copy-on-write mechanism for shared structure

## 📈 **Performance Goals (Post-Optimization)**

### **Target Improvements:**
- **Memory Reduction**: 40-60% reduction in allocations for write operations
- **Write Performance**: 20-30% improvement in set operation speed
- **GC Pressure**: Significant reduction in garbage collection overhead
- **Scaling**: Better performance retention on large datasets

### **Acceptable Trade-offs:**
- Read performance must remain unchanged (0 allocations)
- Immutability guarantees must be preserved
- API compatibility must be maintained

---

*This baseline provides comprehensive metrics for measuring optimization effectiveness* 