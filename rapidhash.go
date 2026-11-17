package immutable

// This file is a port of the rapidhash V3 hash for strings, with the default
// seed, from github.com/dwisiswant0/rapidhash (package go.dw1.io/rapidhash, tag
// v0.3.0, commit 3b3fd64182a9d870a5284b957d296b8feca418a2, file rapidhash.go,
// function Hash).
//
// Changes from the upstream code:
//   - it takes a string, and reads it with explicit little-endian loads instead
//     of unsafe pointer reads, so it needs no unsafe package and gives the same
//     result on every platform (the upstream reads in host byte order);
//   - inputs over 112 bytes always use the portable 7-lane loop (upstream's
//     amd64 assembly accumBlocks only replaces this loop for inputs over 448
//     bytes, and computes the same value).
//
// The algorithm is rapidhash by Nicolas De Carli (github.com/Nicoshev/rapidhash),
// which is based on wyhash by Wang Yi; see those projects for their licenses.
// The Go port it is taken from is distributed under this license:
//
//	Copyright 2026 Dwi Siswanto
//
//	Permission is hereby granted, free of charge, to any person obtaining a copy
//	of this software and associated documentation files (the "Software"), to deal
//	in the Software without restriction, including without limitation the rights
//	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//	copies of the Software, and to permit persons to whom the Software is
//	furnished to do so, subject to the following conditions:
//
//	The above copyright notice and this permission notice shall be included in all
//	copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//	SOFTWARE.

import "math/bits"

// Secret constants of rapidhash.
const (
	rapidSecret0 = 0x2d358dccaa6c78a5
	rapidSecret1 = 0x8bb84b93962eacc9
	rapidSecret2 = 0x4b33a62ed433d4a3
	rapidSecret3 = 0x4d5a2da51de1aa47
	rapidSecret4 = 0xa0761d6478bd642f
	rapidSecret5 = 0xe7037ed1a0b428db
	rapidSecret6 = 0x90ed1765281c388c
	rapidSecret7 = 0xaaaaaaaaaaaaaaaa

	// rapidSeed is the default seed (0) after the initial mixing step
	// seed ^= mix(seed^secret2, secret1).
	rapidSeed = 0x422765567d8fbfd6
)

// rapidMix multiplies a and b to 128 bits and returns the xor of both halves.
func rapidMix(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return lo ^ hi
}

// rapidLoad64 reads the 8 bytes at s[i:] as a little-endian number.
func rapidLoad64(s string, i int) uint64 {
	s = s[i : i+8]
	return uint64(s[0]) | uint64(s[1])<<8 | uint64(s[2])<<16 | uint64(s[3])<<24 |
		uint64(s[4])<<32 | uint64(s[5])<<40 | uint64(s[6])<<48 | uint64(s[7])<<56
}

// rapidLoad32 reads the 4 bytes at s[i:] as a little-endian number.
func rapidLoad32(s string, i int) uint64 {
	s = s[i : i+4]
	return uint64(s[0]) | uint64(s[1])<<8 | uint64(s[2])<<16 | uint64(s[3])<<24
}

// rapidHash returns the 64-bit rapidhash V3 of s with seed 0.
func rapidHash(s string) uint64 {
	n := len(s)
	switch {
	case n == 0:
		hi, lo := bits.Mul64(rapidSecret1, rapidSeed)
		return rapidMix(lo^rapidSecret7, hi^rapidSecret1)
	case n < 4:
		a := uint64(s[0])<<45 | uint64(s[n-1])
		b := uint64(s[n>>1])
		return rapidFinish(a, b, rapidSeed, uint64(n))
	case n < 8:
		return rapidFinish(rapidLoad32(s, 0), rapidLoad32(s, n-4), rapidSeed^uint64(n), uint64(n))
	case n <= 16:
		return rapidFinish(rapidLoad64(s, 0), rapidLoad64(s, n-8), rapidSeed^uint64(n), uint64(n))
	}
	return rapidHashLong(s)
}

// rapidFinish is the last step of every length: it combines the two words a and
// b (already read from the input) with the seed and the length.
func rapidFinish(a, b, seed, n uint64) uint64 {
	a ^= rapidSecret1
	b ^= seed
	hi, lo := bits.Mul64(a, b)
	return rapidMix(lo^rapidSecret7, hi^rapidSecret1^n)
}

// rapidHashLong hashes an input of more than 16 bytes.
func rapidHashLong(s string) uint64 {
	n := len(s)
	seed := uint64(rapidSeed)
	i := n // bytes not yet consumed by the block loops
	off := 0
	if i > 112 {
		see1, see2, see3, see4, see5, see6 := seed, seed, seed, seed, seed, seed
		for ; i > 112; i, off = i-112, off+112 {
			seed = rapidMix(rapidLoad64(s, off)^rapidSecret0, rapidLoad64(s, off+8)^seed)
			see1 = rapidMix(rapidLoad64(s, off+16)^rapidSecret1, rapidLoad64(s, off+24)^see1)
			see2 = rapidMix(rapidLoad64(s, off+32)^rapidSecret2, rapidLoad64(s, off+40)^see2)
			see3 = rapidMix(rapidLoad64(s, off+48)^rapidSecret3, rapidLoad64(s, off+56)^see3)
			see4 = rapidMix(rapidLoad64(s, off+64)^rapidSecret4, rapidLoad64(s, off+72)^see4)
			see5 = rapidMix(rapidLoad64(s, off+80)^rapidSecret5, rapidLoad64(s, off+88)^see5)
			see6 = rapidMix(rapidLoad64(s, off+96)^rapidSecret6, rapidLoad64(s, off+104)^see6)
		}
		seed ^= see1
		see2 ^= see3
		see4 ^= see5
		seed ^= see6
		see2 ^= see4
		seed ^= see2
	}
	if i > 16 {
		seed = rapidMix(rapidLoad64(s, off)^rapidSecret2, rapidLoad64(s, off+8)^seed)
		if i > 32 {
			seed = rapidMix(rapidLoad64(s, off+16)^rapidSecret2, rapidLoad64(s, off+24)^seed)
		}
		if i > 48 {
			seed = rapidMix(rapidLoad64(s, off+32)^rapidSecret1, rapidLoad64(s, off+40)^seed)
		}
		if i > 64 {
			seed = rapidMix(rapidLoad64(s, off+48)^rapidSecret1, rapidLoad64(s, off+56)^seed)
		}
		if i > 80 {
			seed = rapidMix(rapidLoad64(s, off+64)^rapidSecret2, rapidLoad64(s, off+72)^seed)
		}
		if i > 96 {
			seed = rapidMix(rapidLoad64(s, off+80)^rapidSecret1, rapidLoad64(s, off+88)^seed)
		}
	}
	// The last 16 bytes of the whole input, whatever was consumed above.
	a := rapidLoad64(s, n-16) ^ uint64(i)
	b := rapidLoad64(s, n-8)
	a ^= rapidSecret1
	b ^= seed
	hi, lo := bits.Mul64(a, b)
	return rapidMix(lo^rapidSecret7, hi^rapidSecret1^uint64(i))
}
