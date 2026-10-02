package vegscatter

import "math"

// candidate is a potential instance in the global placement. Candidates are
// generated purely from the layer seed and their grid coordinates, so every
// query sees exactly the same candidate stream.
type candidate struct {
	tier    uint8 // 1 = main (random scale), 2 = filler (min scale)
	species int
	cellX   int64
	cellY   int64
	x, y    float64
	scale   float64
	prio    uint64
}

// radius returns the footprint radius of the candidate.
func (c candidate) radius(l *Layer) float64 {
	return l.Species[c.species].BaseRadius * c.scale
}

// gridParams holds derived grid constants for a layer.
type gridParams struct {
	cell1 float64   // main-tier cell size: one candidate per cell
	fill  []float64 // per-species filler-tier cell spacing (0 = disabled)
}

// fillDensity controls how tightly the filler grid is packed relative to the
// species' minimum footprint radius. The saturation guarantee needs the
// maximum gap between filler candidates of a species to stay below half of
// its minimum footprint radius; with full-cell jitter the worst-case gap is
// spacing*sqrt(2), so spacing must be <= 0.5/sqrt(2) ~= 0.3536 radii.
const fillDensity = 0.35

func makeGridParams(l *Layer) gridParams {
	maxR := 0.0
	for _, s := range l.Species {
		if r := s.MaxRadius(); r > maxR {
			maxR = r
		}
	}
	gp := gridParams{cell1: 2 * maxR, fill: make([]float64, len(l.Species))}
	for i, s := range l.Species {
		if s.Weight > 0 && s.MinRadius() > 0 {
			gp.fill[i] = fillDensity * s.MinRadius()
		}
	}
	return gp
}

// drawSpecies picks a species index proportionally to the weights.
func drawSpecies(l *Layer, u float64) int {
	total := 0.0
	for _, s := range l.Species {
		if s.Weight > 0 {
			total += s.Weight
		}
	}
	if total <= 0 {
		return -1
	}
	acc := 0.0
	for i, s := range l.Species {
		if s.Weight <= 0 {
			continue
		}
		acc += s.Weight / total
		if u < acc {
			return i
		}
	}
	return -1
}

// tier1Candidate returns the single main-tier candidate of the given cell.
// Main-tier candidates carry a weight-drawn species and a random scale, and
// always outrank filler-tier candidates (top priority bit set).
func tier1Candidate(h hasher, l *Layer, gp gridParams, cx, cy int64) candidate {
	ux, uy := uint64(cx), uint64(cy)
	sp := drawSpecies(l, h.unit(1, ux, uy, 0))
	jx := h.unit(1, ux, uy, 1)
	jy := h.unit(1, ux, uy, 2)
	scale := 0.0
	if sp >= 0 {
		s := l.Species[sp]
		scale = s.MinScale + h.unit(1, ux, uy, 3)*(s.MaxScale-s.MinScale)
	}
	return candidate{
		tier:    1,
		species: sp,
		cellX:   cx,
		cellY:   cy,
		x:       (float64(cx) + jx) * gp.cell1,
		y:       (float64(cy) + jy) * gp.cell1,
		scale:   scale,
		prio:    h.hash(1, ux, uy, 4) | (1 << 63),
	}
}

// tier2Candidate returns the filler-tier candidate of species sp in the given
// cell of that species' filler grid. Filler candidates sit at the species'
// minimum scale; they guarantee saturation where main-tier candidates left
// room.
func tier2Candidate(h hasher, l *Layer, gp gridParams, sp int, cx, cy int64) candidate {
	g := gp.fill[sp]
	ux, uy, us := uint64(cx), uint64(cy), uint64(sp)
	jx := h.unit(2, us, ux, uy, 0)
	jy := h.unit(2, us, ux, uy, 1)
	return candidate{
		tier:    2,
		species: sp,
		cellX:   cx,
		cellY:   cy,
		x:       (float64(cx) + jx) * g,
		y:       (float64(cy) + jy) * g,
		scale:   l.Species[sp].MinScale,
		prio:    h.hash(2, us, ux, uy, 2) >> 1,
	}
}

// valid reports whether the terrain at the candidate center is legal for the
// candidate's species.
func valid(c candidate, l *Layer, t Terrain) bool {
	if c.species < 0 {
		return false
	}
	s := l.Species[c.species]
	if t.Slope != nil && t.Slope(c.x, c.y) > s.MaxSlope {
		return false
	}
	if t.Height != nil {
		hgt := t.Height(c.x, c.y)
		if hgt < s.MinHeight || hgt > s.MaxHeight {
			return false
		}
	}
	return true
}

// floorDiv returns the index of the grid cell (of the given spacing) that
// contains coordinate v.
func floorDiv(v, spacing float64) int64 {
	return int64(math.Floor(v / spacing))
}
