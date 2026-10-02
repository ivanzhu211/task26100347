package vegscatter

// placer evaluates the global placement lazily. Acceptance of a candidate is
// a pure function of (layer, terrain, candidate identity); the cache only
// memoizes it within a single Query call, so results are independent of call
// order and of previously queried windows.
type placer struct {
	layer   *Layer
	terrain Terrain
	h       hasher
	gp      gridParams
	cache   map[candidate]bool
}

func newPlacer(l *Layer, t Terrain) *placer {
	return &placer{
		layer:   l,
		terrain: t,
		h:       newHasher(l.Seed),
		gp:      makeGridParams(l),
		cache:   make(map[candidate]bool),
	}
}

// outranks reports whether candidate a has strictly higher priority than b.
// Priorities collide with negligible probability; the identity tiebreak keeps
// the order total and deterministic regardless.
func outranks(a, b candidate) bool {
	if a.prio != b.prio {
		return a.prio > b.prio
	}
	if a.tier != b.tier {
		return a.tier > b.tier
	}
	if a.species != b.species {
		return a.species > b.species
	}
	if a.cellX != b.cellX {
		return a.cellX > b.cellX
	}
	return a.cellY > b.cellY
}

// overlaps reports whether the two footprint circles intersect.
func overlaps(a, b candidate, l *Layer) bool {
	r := a.radius(l) + b.radius(l)
	dx := a.x - b.x
	dy := a.y - b.y
	return dx*dx+dy*dy < r*r
}

// accepted reports whether the candidate survives the deterministic greedy
// conflict resolution: a candidate is kept iff it is valid and no accepted
// candidate that outranks it overlaps its footprint. Because priorities
// strictly increase along any dependency chain, the recursion terminates and
// the result is a pure function of the layer, terrain and candidate identity.
func (p *placer) accepted(c candidate) bool {
	if v, ok := p.cache[c]; ok {
		return v
	}
	res := p.resolve(c)
	p.cache[c] = res
	return res
}

func (p *placer) resolve(c candidate) bool {
	if !valid(c, p.layer, p.terrain) {
		return false
	}
	// Main-tier neighbors: every main-tier candidate outranks a filler, and
	// for a main-tier candidate only same-tier candidates can outrank it.
	// Conflicts are impossible beyond radius maxR1+maxR1 = cell1, so the 3x3
	// block of main-tier cells around c always suffices.
	r1 := c.radius(p.layer) + p.gp.cell1/2
	minCX := floorDiv(c.x-r1, p.gp.cell1)
	maxCX := floorDiv(c.x+r1, p.gp.cell1)
	minCY := floorDiv(c.y-r1, p.gp.cell1)
	maxCY := floorDiv(c.y+r1, p.gp.cell1)
	for cx := minCX; cx <= maxCX; cx++ {
		for cy := minCY; cy <= maxCY; cy++ {
			n := tier1Candidate(p.h, p.layer, p.gp, cx, cy)
			if n.species < 0 || n == c || !outranks(n, c) {
				continue
			}
			if overlaps(c, n, p.layer) && p.accepted(n) {
				return false
			}
		}
	}
	if c.tier == 1 {
		return true
	}
	// Filler-tier neighbors: only filler candidates that outrank c can block
	// it. Species with a disabled filler grid contribute nothing.
	for sp, g := range p.gp.fill {
		if g <= 0 {
			continue
		}
		r := c.radius(p.layer) + p.layer.Species[sp].MinRadius()
		minFX := floorDiv(c.x-r, g)
		maxFX := floorDiv(c.x+r, g)
		minFY := floorDiv(c.y-r, g)
		maxFY := floorDiv(c.y+r, g)
		for cx := minFX; cx <= maxFX; cx++ {
			for cy := minFY; cy <= maxFY; cy++ {
				n := tier2Candidate(p.h, p.layer, p.gp, sp, cx, cy)
				if n == c || !outranks(n, c) {
					continue
				}
				if overlaps(c, n, p.layer) && p.accepted(n) {
					return false
				}
			}
		}
	}
	return true
}

// collect appends accepted candidates whose centers fall inside window.
func (p *placer) collect(window Rect, out []Instance) []Instance {
	if p.gp.cell1 > 0 {
		minCX := floorDiv(window.MinX, p.gp.cell1)
		maxCX := floorDiv(window.MaxX, p.gp.cell1)
		minCY := floorDiv(window.MinY, p.gp.cell1)
		maxCY := floorDiv(window.MaxY, p.gp.cell1)
		for cx := minCX; cx <= maxCX; cx++ {
			for cy := minCY; cy <= maxCY; cy++ {
				c := tier1Candidate(p.h, p.layer, p.gp, cx, cy)
				if c.species >= 0 && window.Contains(c.x, c.y) && p.accepted(c) {
					out = append(out, Instance{Species: c.species, X: c.x, Y: c.y, Scale: c.scale})
				}
			}
		}
	}
	for sp, g := range p.gp.fill {
		if g <= 0 {
			continue
		}
		minCX := floorDiv(window.MinX, g)
		maxCX := floorDiv(window.MaxX, g)
		minCY := floorDiv(window.MinY, g)
		maxCY := floorDiv(window.MaxY, g)
		for cx := minCX; cx <= maxCX; cx++ {
			for cy := minCY; cy <= maxCY; cy++ {
				c := tier2Candidate(p.h, p.layer, p.gp, sp, cx, cy)
				if window.Contains(c.x, c.y) && p.accepted(c) {
					out = append(out, Instance{Species: c.species, X: c.x, Y: c.y, Scale: c.scale})
				}
			}
		}
	}
	return out
}
