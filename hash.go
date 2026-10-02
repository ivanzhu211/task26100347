package vegscatter

// mix64 is the splitmix64 finalizer: a strong 64-bit bit mixer.
func mix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// hashCell deterministically hashes the layer seed together with integer
// cell coordinates and an attempt counter.
func hashCell(seed uint64, i, j int64, k uint64) uint64 {
	h := mix64(seed ^ 0x9E3779B97F4A7C15)
	h = mix64(h ^ uint64(i))
	h = mix64(h ^ uint64(j)*0x9E3779B97F4A7C15)
	h = mix64(h ^ k*0xC2B2AE3D27D4EB4F)
	return h
}

// unit maps a hash to [0, 1) using its top 53 bits.
func unit(h uint64) float64 {
	return float64(h>>11) * (1.0 / (1 << 53))
}
