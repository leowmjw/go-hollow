package internal

import (
	"hash/fnv"
)

// Hash64String returns a stable 64-bit hash for the given string using FNV-1a 64-bit.
// Pure Go, fast, and non-cryptographic. Suitable for PK-stable sharding.
func Hash64String(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
