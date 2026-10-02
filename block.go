package vegscatter

import "math"

// gridParams holds the derived constants of the global placement scheme.
//
// The plane is partitioned into square blocks of side block = 2*maxRadius.
// Blocks are colored with a 3x3 phase pattern, so two blocks of the same
// phase are always at least 2*block apart and their instances can never
// overlap. Phases are resolved in a fixed order; inside a block,
// candidate points of a global sub-lattice (spacing step) are resolved
// sequentially. Everything is derived from hashes of global indices, so
// the placement is independent of any query window.
type gridParams struct {
	block float64 // block size in meters: 2*maxRadius
	step  float64 // sub-lattice spacing in meters
	maxR  float64 // maximum occupancy radius over all species
	haloB int     // dependency halo in blocks (phases-1 = 8)
}

const (
	phasePeriod = 3
	phaseCount  = phasePeriod * phasePeriod
)

// deriveGrid computes the grid constants for the layer.
func deriveGrid(l Layer) gridParams {
	minR := math.Inf(1)
	maxR := 0.0
	for _, s := range l.Species {
		if r := s.MinRadius(); r < minR {
			minR = r
		}
		if r := s.MaxRadius(); r > maxR {
			maxR = r
		}
	}
	return gridParams{
		block: 2 * maxR,
		step:  minR / 1.5,
		maxR:  maxR,
		haloB: phaseCount - 1,
	}
}

// candidate is one placement attempt at one lattice point.
type candidate struct {
	species int
	x, y    float64
	scale   float64
	radius  float64
}

// forEachCandidate calls fn for every placement attempt of the lattice
// points inside block (bi, bj), in a deterministic order, until fn
// returns true (accepted). The first attempt at each point uses a
// weighted species pick, a random scale and a small jitter; the remaining
// attempts retry every species at its minimum scale exactly on the
// lattice point, which is what guarantees saturation.
func forEachCandidate(l Layer, g gridParams, bi, bj int64, fn func(candidate) bool) {
	n := len(l.Species)
	totalW := 0.0
	for _, s := range l.Species {
		totalW += s.Weight
	}

	loX := float64(bi) * g.block
	hiX := loX + g.block
	loY := float64(bj) * g.block
	hiY := loY + g.block

	for m := latticeFirst(loX, g.step); ; m++ {
		px := (float64(m) + 0.5) * g.step
		if px >= hiX {
			break
		}
		for k := latticeFirst(loY, g.step); ; k++ {
			py := (float64(k) + 0.5) * g.step
			if py >= hiY {
				break
			}

			// Attempt 0: weighted species, random scale, jitter.
			h0 := hashCell(l.Seed, m, k, 0)
			pick := unit(h0) * totalW
			sp := n - 1
			acc := 0.0
			for idx, s := range l.Species {
				acc += s.Weight
				if pick < acc {
					sp = idx
					break
				}
			}
			spec := l.Species[sp]
			scale := spec.MinScale + unit(hashCell(l.Seed, m, k, 1))*(spec.MaxScale-spec.MinScale)
			jx := (unit(hashCell(l.Seed, m, k, 2))*2 - 1) * 0.2 * g.step
			jy := (unit(hashCell(l.Seed, m, k, 3))*2 - 1) * 0.2 * g.step
			if fn(candidate{species: sp, x: px + jx, y: py + jy, scale: scale, radius: spec.BaseRadius * scale}) {
				continue
			}

			// Retries: every species at minimum scale, on the lattice point.
			start := int(hashCell(l.Seed, m, k, 4) % uint64(n))
			for r := 0; r < n; r++ {
				idx := (start + r) % n
				s := l.Species[idx]
				if fn(candidate{species: idx, x: px, y: py, scale: s.MinScale, radius: s.MinRadius()}) {
					break
				}
			}
		}
	}
}

// latticeFirst returns the smallest lattice index whose point is >= lo.
func latticeFirst(lo, step float64) int64 {
	return int64(math.Ceil(lo/step - 0.5))
}

// legal reports whether the species may be placed at (x, y).
func legal(t Terrain, s Species, x, y float64) bool {
	if t.Slope != nil && t.Slope(x, y) > s.MaxSlope {
		return false
	}
	if t.Height != nil {
		h := t.Height(x, y)
		if h < s.MinElev || h > s.MaxElev {
			return false
		}
	}
	return true
}
