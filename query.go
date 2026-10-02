package vegscatter

import (
	"math"
	"sort"
)

// bucketGrid is a dense spatial index over the evaluated region.
type bucketGrid struct {
	size       float64 // bucket side in meters
	ox, oy     float64 // origin of bucket (0,0)
	w, h       int
	head       []int // bucket -> first entry index, -1 if empty
	next       []int // entry -> next entry in same bucket
	xs, ys, rs []float64
	maxR       float64
}

func newBucketGrid(minX, minY, maxX, maxY, size, maxR float64) *bucketGrid {
	w := int(math.Ceil((maxX-minX)/size)) + 1
	h := int(math.Ceil((maxY-minY)/size)) + 1
	head := make([]int, w*h)
	for i := range head {
		head[i] = -1
	}
	return &bucketGrid{size: size, ox: minX, oy: minY, w: w, h: h, head: head, maxR: maxR}
}

func (b *bucketGrid) add(x, y, r float64) {
	idx := len(b.xs)
	b.xs = append(b.xs, x)
	b.ys = append(b.ys, y)
	b.rs = append(b.rs, r)
	bi := int((x - b.ox) / b.size)
	bj := int((y - b.oy) / b.size)
	if bi < 0 {
		bi = 0
	}
	if bi >= b.w {
		bi = b.w - 1
	}
	if bj < 0 {
		bj = 0
	}
	if bj >= b.h {
		bj = b.h - 1
	}
	k := bj*b.w + bi
	b.next = append(b.next, b.head[k])
	b.head[k] = idx
}

// overlaps reports whether a circle of radius r at (x, y) intersects any
// inserted circle.
func (b *bucketGrid) overlaps(x, y, r float64) bool {
	reach := int(math.Ceil((r+b.maxR)/b.size)) + 1
	ci := int((x - b.ox) / b.size)
	cj := int((y - b.oy) / b.size)
	for dj := -reach; dj <= reach; dj++ {
		jj := cj + dj
		if jj < 0 || jj >= b.h {
			continue
		}
		for di := -reach; di <= reach; di++ {
			ii := ci + di
			if ii < 0 || ii >= b.w {
				continue
			}
			for e := b.head[jj*b.w+ii]; e >= 0; e = b.next[e] {
				dx := x - b.xs[e]
				dy := y - b.ys[e]
				rr := r + b.rs[e]
				if dx*dx+dy*dy < rr*rr {
					return true
				}
			}
		}
	}
	return false
}

// Query returns all instances of the global deterministic placement whose
// centers fall inside window w. The result depends only on the layer, the
// terrain and the window: Query evaluates a fixed, window-independent
// acceptance procedure on the global block grid and then clips to w.
func Query(l Layer, t Terrain, w Rect) []Instance {
	if len(l.Species) == 0 || w.MaxX <= w.MinX || w.MaxY <= w.MinY {
		return nil
	}
	g := deriveGrid(l)

	bi0 := int64(math.Floor(w.MinX/g.block)) - int64(g.haloB)
	bi1 := int64(math.Floor(w.MaxX/g.block)) + int64(g.haloB)
	bj0 := int64(math.Floor(w.MinY/g.block)) - int64(g.haloB)
	bj1 := int64(math.Floor(w.MaxY/g.block)) + int64(g.haloB)

	minX := float64(bi0) * g.block
	minY := float64(bj0) * g.block
	maxX := float64(bi1+1) * g.block
	maxY := float64(bj1+1) * g.block
	grid := newBucketGrid(minX, minY, maxX, maxY, g.maxR, g.maxR)

	var accepted []Instance

	// Resolve the 9 block phases in a fixed order. Blocks of one phase
	// never interact, and each phase only reads instances accepted by
	// earlier phases, so the outcome is identical for every query window.
	for ph := 0; ph < phaseCount; ph++ {
		a := ph % phasePeriod
		b := ph / phasePeriod
		for bj := alignBlock(bj0, b); bj <= bj1; bj += phasePeriod {
			for bi := alignBlock(bi0, a); bi <= bi1; bi += phasePeriod {
				forEachCandidate(l, g, bi, bj, func(c candidate) bool {
					if !legal(t, l.Species[c.species], c.x, c.y) {
						return false
					}
					if grid.overlaps(c.x, c.y, c.radius) {
						return false
					}
					grid.add(c.x, c.y, c.radius)
					accepted = append(accepted, Instance{Species: c.species, X: c.x, Y: c.y, Scale: c.scale})
					return true
				})
			}
		}
	}

	out := make([]Instance, 0, len(accepted)/4)
	for _, in := range accepted {
		if w.Contains(in.X, in.Y) {
			out = append(out, in)
		}
	}
	sort.Slice(out, func(p, q int) bool {
		if out[p].X != out[q].X {
			return out[p].X < out[q].X
		}
		if out[p].Y != out[q].Y {
			return out[p].Y < out[q].Y
		}
		if out[p].Species != out[q].Species {
			return out[p].Species < out[q].Species
		}
		return out[p].Scale < out[q].Scale
	})
	return out
}

// alignBlock returns the smallest block index >= start congruent to r
// modulo phasePeriod.
func alignBlock(start int64, r int) int64 {
	m := ((start % phasePeriod) + phasePeriod) % phasePeriod
	return start + (int64(r)-m+phasePeriod)%phasePeriod
}
