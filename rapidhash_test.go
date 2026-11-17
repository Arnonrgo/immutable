package immutable

import (
	"fmt"
	"strings"
	"testing"
)

// rapidGolden are the rapidhash V3 values (seed 0) of the first n bytes of the
// sequence byte(i*167+29), computed with go.dw1.io/rapidhash v0.0.0-3b3fd64 (tag
// v0.3.0). They cover every branch of the algorithm: lengths 0 to 3, 4 to 7, 8 to
// 16, 17 to 112 at every 16-byte step, and the 112-byte block loop (113, 224,
// 448 and above).
var rapidGolden = []struct {
	n    int
	hash uint64
}{
	{0, 0x0338dc4be2cecdae},
	{1, 0x37837732332acfdb},
	{2, 0xfe4f11ebdcd7de0c},
	{3, 0x8fe09247f85fc088},
	{4, 0xcbfe9e58e7f38f12},
	{5, 0xfbd57025cd4a81ad},
	{6, 0xfbb47b71dfb4e4db},
	{7, 0xc86e85dc0e55042f},
	{8, 0xd3b2d31af25c078a},
	{9, 0x7320ca4a120efffa},
	{10, 0x531ee48e87a61ee6},
	{11, 0x640fb3dd2c8919af},
	{12, 0xcc708cbc161db3f3},
	{13, 0xdacefdeb278afad9},
	{14, 0x0dbe2ad5391cbaf5},
	{15, 0xa75f184fd1271617},
	{16, 0xb2284e4148de5bdf},
	{17, 0x4d85c2222a55fb56},
	{18, 0x14528453ebea683b},
	{19, 0xb666c72dc324a34c},
	{20, 0x38eae501ab402269},
	{21, 0x1769dae2b904f87d},
	{22, 0xb32868e204aa30d3},
	{23, 0xd955cbe1afc17bf4},
	{24, 0xe7b5dcbde1fedf8c},
	{25, 0xc9f70e3216e9a9ab},
	{26, 0x3587089d83b33b6d},
	{27, 0x38999f07ae4d830a},
	{28, 0x5930918c2bfde57f},
	{29, 0x068bb46501913948},
	{30, 0x9ea462a5e1d38dc1},
	{31, 0xe8946d3d0abd9bfa},
	{32, 0xb614cd449a4a1902},
	{33, 0x1fab214b59c01521},
	{34, 0xeb41a5b09d50118b},
	{35, 0xe58fc27d5d297c8d},
	{36, 0xe1c7edf5de0cf900},
	{37, 0xad3ba3e46f74ac04},
	{38, 0x32975f09eccc97e7},
	{39, 0x2fad9813dd472b99},
	{40, 0x248601cb641c6aea},
	{47, 0xbda3d3a7d0faf4ee},
	{48, 0xa36d08c6d5f49665},
	{49, 0x87a58f884a63bb49},
	{63, 0x2e6dc6f3da4fcd8b},
	{64, 0xf05dc43fc0915e7c},
	{65, 0x4b3317ee47a91db2},
	{79, 0x4fbd3a6cd6943a63},
	{80, 0xaef83f980bfd4308},
	{81, 0xa9a89d59a8ff98eb},
	{95, 0x307c9255183dffa5},
	{96, 0x14311c2352e93f97},
	{97, 0x7aef26ca164665d7},
	{111, 0xb445e8587c861874},
	{112, 0x5ace7a1ce7463c2b},
	{113, 0x0ed6e63d45b1a4c1},
	{127, 0x5733a34de02dd1c0},
	{128, 0x849625c030a3fede},
	{129, 0x162c5346828fcccd},
	{223, 0x822529fbd11e88ce},
	{224, 0xac62af17a9acaa3f},
	{225, 0x6c02bad2765092b5},
	{336, 0x73f5b0a79816c0b8},
	{337, 0x40409cccb805fa08},
	{447, 0x8653b5fa091494ba},
	{448, 0xd903044d46007e3f},
	{449, 0x7d40959a182c1587},
	{450, 0x11e1e681f3d8e417},
	{511, 0xf73962323177b8de},
	{512, 0x0ebe887210c9a318},
	{1000, 0x811985ff11dc2e28},
	{1024, 0x8c2e01fd98cd1af4},
	{4096, 0xf35f2a6f4cfa1353},
}

func TestRapidHashMatchesUpstream(t *testing.T) {
	for _, g := range rapidGolden {
		b := make([]byte, g.n)
		for i := range b {
			b[i] = byte(i*167 + 29)
		}
		if got := rapidHash(string(b)); got != g.hash {
			t.Errorf("rapidHash of %d bytes = %#016x, want %#016x", g.n, got, g.hash)
		}
	}
}

// polynomialStringHash is the string hash this package used before rapidhash:
// hash = 31*hash + byte. It stays here as the baseline the tests compare with.
func polynomialStringHash(s string) uint32 {
	var h uint32
	for i := 0; i < len(s); i++ {
		h = 31*h + uint32(s[i])
	}
	return h
}

// polynomialHasher is a Hasher[string] over polynomialStringHash, for building
// the baseline maps.
type polynomialHasher struct{}

func (polynomialHasher) Hash(s string) uint32   { return polynomialStringHash(s) }
func (polynomialHasher) Equal(a, b string) bool { return a == b }

// TestRapidHashReadsEveryByte flips each bit of inputs of every length class and
// requires a different hash, so no byte is skipped or read twice by mistake
// (the long path reads overlapping words at the end, and a wrong offset would
// leave a middle byte out).
func TestRapidHashReadsEveryByte(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 7, 8, 15, 16, 17, 31, 32, 33, 48, 64, 80, 96, 97, 111, 112, 113, 127, 128, 200, 224, 225, 449} {
		base := make([]byte, n)
		for i := range base {
			base[i] = byte(i*167 + 29)
		}
		want := rapidHash(string(base))
		for i := 0; i < n; i++ {
			for bit := 0; bit < 8; bit++ {
				base[i] ^= 1 << bit
				if got := rapidHash(string(base)); got == want {
					t.Errorf("len %d: flipping bit %d of byte %d did not change the hash", n, bit, i)
				}
				base[i] ^= 1 << bit
			}
		}
	}
}

// TestDefaultStringHashSeparatesPolynomialCollisions: "Aa" and "BB" have the
// same polynomial hash (65*31+97 = 66*31+66), and so does every string made by
// joining them, so 1,024 strings of 10 blocks shared one hash and the map held
// them in a single collision node, scanned linearly. rapidhash keeps them apart.
func TestDefaultStringHashSeparatesPolynomialCollisions(t *testing.T) {
	const blocks = 10
	keys := make([]string, 0, 1<<blocks)
	for mask := 0; mask < 1<<blocks; mask++ {
		var sb strings.Builder
		for b := 0; b < blocks; b++ {
			if mask>>b&1 == 0 {
				sb.WriteString("Aa")
			} else {
				sb.WriteString("BB")
			}
		}
		keys = append(keys, sb.String())
	}

	old := map[uint32]struct{}{}
	cur := map[uint32]struct{}{}
	for _, k := range keys {
		old[polynomialStringHash(k)] = struct{}{}
		cur[hashString(k)] = struct{}{}
	}
	if len(old) != 1 {
		t.Fatalf("baseline changed: the polynomial hash gives %d distinct values for the Aa/BB family, want 1", len(old))
	}
	if len(cur) != len(keys) {
		t.Errorf("hashString gives %d distinct values for %d Aa/BB strings, want all distinct", len(cur), len(keys))
	}

	b := NewMapBuilder[string, int](nil)
	for i, k := range keys {
		b.Set(k, i)
	}
	m := b.Map()
	if s := trieStats(m); s.collisions != 0 || s.maxDepth > 4 {
		t.Errorf("Aa/BB map has %d collision nodes and depth %d, want 0 and at most 4", s.collisions, s.maxDepth)
	}
	for i, k := range keys {
		if v, ok := m.Get(k); !ok || v != i {
			t.Fatalf("Get(%q) = %d, %v; want %d, true", k, v, ok, i)
		}
	}
}

// realisticStringKeys returns n keys of the kinds programs use: counters with a
// prefix, uuids, file paths and short words.
func realisticStringKeys() map[string][]string {
	const n = 100_000
	rng := splitmix(7)
	hex := func(k int) string {
		const digits = "0123456789abcdef"
		b := make([]byte, k)
		for i := range b {
			b[i] = digits[rng.intn(16)]
		}
		return string(b)
	}
	sets := map[string][]string{}
	for i := 1; i <= n; i++ {
		sets["key counter"] = append(sets["key counter"], fmt.Sprintf("key%d", i))
		sets["uuid"] = append(sets["uuid"], hex(8)+"-"+hex(4)+"-"+hex(4)+"-"+hex(4)+"-"+hex(12))
		sets["path"] = append(sets["path"], fmt.Sprintf("/var/lib/app/%d/%d/%s.log", i%97, i/97, hex(3)))
		sets["short"] = append(sets["short"], hex(1+rng.intn(7)))
	}
	// "short" can repeat; the others are unique by construction (uuid could in
	// theory, at 2^-100 odds).
	seen := map[string]struct{}{}
	uniq := sets["short"][:0]
	for _, k := range sets["short"] {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			uniq = append(uniq, k)
		}
	}
	sets["short"] = uniq
	return sets
}

// fullHashCollisions counts the keys whose 32-bit hash equals the hash of
// another key (every key of a group of size g counts g-1).
func fullHashCollisions(keys []string, hash func(string) uint32) int {
	seen := make(map[uint32]struct{}, len(keys))
	dup := 0
	for _, k := range keys {
		h := hash(k)
		if _, ok := seen[h]; ok {
			dup++
		}
		seen[h] = struct{}{}
	}
	return dup
}

// TestDefaultStringHashSpreadsRealisticKeys checks that the default string hash
// behaves like a random 32-bit function on keys programs actually use: about
// n^2/2^33 full-hash collisions (1.2 for 100,000 keys) and a trie as shallow as
// a random one's. The same keys through the old polynomial hash are logged for
// comparison.
func TestDefaultStringHashSpreadsRealisticKeys(t *testing.T) {
	for name, keys := range realisticStringKeys() {
		t.Run(name, func(t *testing.T) {
			n := len(keys)
			// A random 32-bit hash collides about n*(n-1)/2^33 times; allow 8
			// times that and at least 8, so a fixed key set cannot fail by bad luck.
			expected := float64(n) * float64(n-1) / (1 << 33)
			limit := max(8, int(8*expected))
			cur := fullHashCollisions(keys, hashString)
			t.Logf("%s: %d keys, full-hash collisions: rapidhash %d, polynomial %d (random: %.1f)",
				name, n, cur, fullHashCollisions(keys, polynomialStringHash), expected)
			if cur > limit {
				t.Errorf("%d full 32-bit hash collisions among %d keys, want at most %d", cur, n, limit)
			}

			b := NewMapBuilder[string, int](nil)
			ob := NewMapBuilder[string, int](polynomialHasher{})
			for i, k := range keys {
				b.Set(k, i)
				ob.Set(k, i)
			}
			m := b.Map()
			s := trieStats(m)
			os := trieStats(ob.Map())
			t.Logf("%s: rapidhash values=%d nodes=%d maxDepth=%d collisionNodes=%d; polynomial nodes=%d maxDepth=%d collisionNodes=%d",
				name, s.values, s.nodes, s.maxDepth, s.collisions, os.nodes, os.maxDepth, os.collisions)
			if s.values != n {
				t.Fatalf("map holds %d values, want %d", s.values, n)
			}
			// A random hash needs about n/3 nodes for large n and a depth of 6 or 7;
			// a skewed one needs many more nodes or a deeper trie.
			if s.maxDepth > 8 {
				t.Errorf("trie depth %d, want at most 8", s.maxDepth)
			}
			if s.nodes > n/2 {
				t.Errorf("trie has %d nodes for %d keys, want at most %d", s.nodes, n, n/2)
			}
			if s.collisions > limit {
				t.Errorf("trie has %d collision nodes, want at most %d", s.collisions, limit)
			}
		})
	}
}

// TestDefaultStringHashIsStable pins hashString to fixed values (the upstream
// rapidhash of each string, folded to 32 bits), so a change of the hash (which changes the iteration order of every string-keyed Map) shows
// up in review as an edit of this test, and the order stays the same in every
// process and on every platform.
func TestDefaultStringHashIsStable(t *testing.T) {
	for _, c := range []struct {
		in   string
		want uint32
	}{
		{"", 0xe1f611e5},
		{"a", 0x6a3da634},
		{"key1", 0x20d97cce},
		{"Aa", 0xd4527b6d},
		{"BB", 0x25fce126},
		{"hello, world", 0x76705395},
		{"The quick brown fox jumps over the lazy dog", 0x445812b3},
	} {
		if got := hashString(c.in); got != c.want {
			t.Errorf("hashString(%q) = %#08x, want %#08x", c.in, got, c.want)
		}
	}
	// The same string always gives the same hash, whether the map holds it as a
	// key of the default hasher or of the reflection fallback (a named string type).
	type name string
	rh := NewHasher(name(""))
	for _, s := range []string{"", "a", "key1", "Aa", "BB", strings.Repeat("x", 300)} {
		if got, want := rh.Hash(name(s)), hashString(s); got != want {
			t.Errorf("reflection hasher: Hash(%q) = %#08x, want %#08x", s, got, want)
		}
		if got, want := NewHasher("").Hash(s), hashString(s); got != want {
			t.Errorf("string hasher: Hash(%q) = %#08x, want %#08x", s, got, want)
		}
	}
}

var sinkHash uint32

// BenchmarkHashString compares the default string hash with the polynomial hash
// it replaced, on keys of 8, 16, 64 and 1,024 bytes.
func BenchmarkHashString(b *testing.B) {
	for _, n := range []int{8, 16, 64, 1024} {
		key := make([]byte, n)
		for i := range key {
			key[i] = byte(i*167 + 29)
		}
		s := string(key)
		b.Run(fmt.Sprintf("rapidhash/%dB", n), func(b *testing.B) {
			b.SetBytes(int64(n))
			var h uint32
			for i := 0; i < b.N; i++ {
				h += hashString(s)
			}
			sinkHash = h
		})
		b.Run(fmt.Sprintf("polynomial/%dB", n), func(b *testing.B) {
			b.SetBytes(int64(n))
			var h uint32
			for i := 0; i < b.N; i++ {
				h += polynomialStringHash(s)
			}
			sinkHash = h
		})
	}
}
