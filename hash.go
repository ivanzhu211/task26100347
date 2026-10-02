package vegscatter

// hasher derives deterministic pseudo-random values from the layer seed and
// a candidate identity. All arithmetic is integer-based and therefore
// identical across platforms and runs.
type hasher struct {
	seed uint64
}

func newHasher(seed uint64) hasher {
	return hasher{seed: seed}
}

// mix64 is the splitmix64 finalizer.
func mix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// hash mixes the seed with the given words.
func (h hasher) hash(words ...uint64) uint64 {
	z := h.seed + 0x9E3779B97F4A7C15
	for _, w := range words {
		z ^= mix64(w + 0x9E3779B97F4A7C15)
		z = z*0x100000001B3 + 0x9E3779B97F4A7C15
	}
	return mix64(z)
}

// unit returns a deterministic value in [0, 1) for the given words.
func (h hasher) unit(words ...uint64) float64 {
	// Use the top 53 bits as the mantissa of a float in [0, 1).
	return float64(h.hash(words...)>>11) * (1.0 / (1 << 53))
}
