package vegscatter

import (
	"math"
	"sort"
	"testing"
)

func testSpecies() []Species {
	return []Species{
		{Name: "grass", BaseRadius: 1.0, Weight: 3, MinScale: 1.0, MaxScale: 1.2, MaxSlope: 30, MinElev: -1000, MaxElev: 1000},
		{Name: "bush", BaseRadius: 1.2, Weight: 2, MinScale: 1.0, MaxScale: 1.2, MaxSlope: 15, MinElev: -1000, MaxElev: 1000},
		{Name: "alpine", BaseRadius: 1.0, Weight: 1, MinScale: 1.0, MaxScale: 1.1, MaxSlope: 45, MinElev: 5, MaxElev: 1000},
	}
}

func testTerrain() Terrain {
	return Terrain{
		Height: func(x, y float64) float64 {
			return 8*math.Sin(0.03*x)*math.Cos(0.021*y) + 0.02*x
		},
		Slope: func(x, y float64) float64 {
			gx := 0.24*math.Cos(0.03*x)*math.Cos(0.021*y) + 0.02
			gy := -0.168 * math.Sin(0.03*x) * math.Sin(0.021*y)
			return math.Atan(math.Hypot(gx, gy)) * 180 / math.Pi
		},
	}
}

type scenario struct {
	name  string
	layer Layer
	big   Rect
}

func scenarios() []scenario {
	return []scenario{
		{name: "main", layer: Layer{Seed: 20261003, Species: testSpecies()}, big: Rect{MinX: 13.7, MinY: -41.2, MaxX: 13.7 + 60, MaxY: -41.2 + 60}},
		{name: "shifted", layer: Layer{Seed: 777, Species: testSpecies()}, big: Rect{MinX: -120.35, MinY: 55.83, MaxX: -120.35 + 60, MaxY: 55.83 + 60}},
	}
}

// splitRect cuts r into a 3x3 tiling along deliberately arbitrary lines.
func splitRect(r Rect) []Rect {
	xs := []float64{r.MinX, r.MinX + 18.73, r.MinX + 41.31, r.MaxX}
	ys := []float64{r.MinY, r.MinY + 17.19, r.MinY + 43.07, r.MaxY}
	var out []Rect
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			out = append(out, Rect{MinX: xs[a], MaxX: xs[a+1], MinY: ys[b], MaxY: ys[b+1]})
		}
	}
	return out
}

func sortInstances(in []Instance) {
	sort.Slice(in, func(p, q int) bool {
		if in[p].X != in[q].X {
			return in[p].X < in[q].X
		}
		if in[p].Y != in[q].Y {
			return in[p].Y < in[q].Y
		}
		if in[p].Species != in[q].Species {
			return in[p].Species < in[q].Species
		}
		return in[p].Scale < in[q].Scale
	})
}

// queryTiles queries the 3x3 tiling of big and returns the union.
func queryTiles(l Layer, terrain Terrain, big Rect) []Instance {
	var union []Instance
	for _, tile := range splitRect(big) {
		union = append(union, Query(l, terrain, tile)...)
	}
	sortInstances(union)
	return union
}

// TestTiledUnionMatchesBigWindow queries a big window and a 3x3 tiling of
// it; the union of the tiles must be bitwise identical to the big query.
func TestTiledUnionMatchesBigWindow(t *testing.T) {
	terrain := testTerrain()
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			want := Query(sc.layer, terrain, sc.big)
			union := queryTiles(sc.layer, terrain, sc.big)
			if len(want) != len(union) {
				t.Fatalf("instance count differs: big=%d union=%d", len(want), len(union))
			}
			for i := range want {
				if want[i] != union[i] {
					t.Fatalf("instance %d differs: big=%+v union=%+v", i, want[i], union[i])
				}
			}
		})
	}
}

// TestDeterministic repeats the same query and expects identical output.
func TestDeterministic(t *testing.T) {
	terrain := testTerrain()
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			a := Query(sc.layer, terrain, sc.big)
			b := Query(sc.layer, terrain, sc.big)
			if len(a) != len(b) {
				t.Fatalf("count differs between runs: %d vs %d", len(a), len(b))
			}
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("run differs at %d: %+v vs %+v", i, a[i], b[i])
				}
			}
		})
	}
}

// spatialIndex buckets instances for fast neighborhood queries in tests.
type spatialIndex struct {
	cell  float64
	items map[[2]int][]Instance
	layer Layer
}

func newSpatialIndex(l Layer, cell float64) *spatialIndex {
	return &spatialIndex{cell: cell, items: make(map[[2]int][]Instance), layer: l}
}

func (s *spatialIndex) add(in Instance) {
	k := [2]int{int(math.Floor(in.X / s.cell)), int(math.Floor(in.Y / s.cell))}
	s.items[k] = append(s.items[k], in)
}

// forEachNear calls fn for every instance whose circle may interact with a
// circle of radius r at (x, y).
func (s *spatialIndex) forEachNear(x, y, r float64, fn func(Instance)) {
	maxR := 0.0
	for _, sp := range s.layer.Species {
		if sp.MaxRadius() > maxR {
			maxR = sp.MaxRadius()
		}
	}
	reach := int(math.Ceil((r+maxR)/s.cell)) + 1
	ci := int(math.Floor(x / s.cell))
	cj := int(math.Floor(y / s.cell))
	for dj := -reach; dj <= reach; dj++ {
		for di := -reach; di <= reach; di++ {
			for _, in := range s.items[[2]int{ci + di, cj + dj}] {
				fn(in)
			}
		}
	}
}

// TestNoOverlap verifies pairwise non-overlap over the union of tiles.
func TestNoOverlap(t *testing.T) {
	terrain := testTerrain()
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			all := queryTiles(sc.layer, terrain, sc.big)
			idx := newSpatialIndex(sc.layer, 4)
			for _, in := range all {
				idx.add(in)
			}
			for _, in := range all {
				r1 := in.Radius(sc.layer)
				idx.forEachNear(in.X, in.Y, r1, func(o Instance) {
					if o == in {
						return
					}
					r2 := o.Radius(sc.layer)
					d := math.Hypot(in.X-o.X, in.Y-o.Y)
					if d < r1+r2 {
						t.Fatalf("overlap: %+v (r=%.3f) vs %+v (r=%.3f), dist=%.4f", in, r1, o, r2, d)
					}
				})
			}
		})
	}
}

// TestSaturation samples the window (with extra density along the tile
// seams) and verifies that no legal empty spot remains: wherever a species
// is legal, a disc of 1.5x its minimum radius must overlap some instance.
func TestSaturation(t *testing.T) {
	terrain := testTerrain()
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			big := sc.big
			all := queryTiles(sc.layer, terrain, big)
			idx := newSpatialIndex(sc.layer, 4)
			for _, in := range all {
				idx.add(in)
			}

			const slopeMargin = 1.0
			const elevMargin = 0.5
			legalWithMargin := func(sp Species, x, y float64) bool {
				if terrain.Slope(x, y) > sp.MaxSlope-slopeMargin {
					return false
				}
				h := terrain.Height(x, y)
				return h >= sp.MinElev+elevMargin && h <= sp.MaxElev-elevMargin
			}

			// Saturation is a property of the global placement; near the
			// outer window edge a covering instance's center may fall
			// outside the queried window, so only verify points
			// sufficiently far inside.
			const edge = 3.5
			inner := Rect{MinX: big.MinX + edge, MinY: big.MinY + edge, MaxX: big.MaxX - edge, MaxY: big.MaxY - edge}

			check := func(x, y float64) {
				for si, sp := range sc.layer.Species {
					if !legalWithMargin(sp, x, y) {
						continue
					}
					reach := 1.5 * sp.MinRadius()
					covered := false
					idx.forEachNear(x, y, reach, func(o Instance) {
						if covered {
							return
						}
						d := math.Hypot(x-o.X, y-o.Y)
						if d < reach+o.Radius(sc.layer) {
							covered = true
						}
					})
					if !covered {
						t.Fatalf("saturation violated at (%.2f, %.2f): species %q (minR=%.2f) has a free 1.5x spot",
							x, y, sc.layer.Species[si].Name, sp.MinRadius())
					}
				}
			}

			// Dense sampling over the inner window.
			for y := inner.MinY + 0.25; y < inner.MaxY; y += 0.5 {
				for x := inner.MinX + 0.25; x < inner.MaxX; x += 0.5 {
					check(x, y)
				}
			}

			// Extra sampling straddling the seam lines (all interior).
			seams := map[float64]bool{}
			for _, r := range splitRect(big) {
				if r.MinX > big.MinX && r.MinX < big.MaxX {
					seams[r.MinX] = true
				}
				if r.MinY > big.MinY && r.MinY < big.MaxY {
					seams[r.MinY] = true
				}
			}
			for s := range seams {
				for off := -2.0; off <= 2.0; off += 0.5 {
					for t0 := inner.MinY + 0.5; t0 < inner.MaxY; t0 += 1.0 {
						if x := s + off; x >= inner.MinX && x < inner.MaxX {
							check(x, t0)
						}
					}
					for t0 := inner.MinX + 0.5; t0 < inner.MaxX; t0 += 1.0 {
						if y := s + off; y >= inner.MinY && y < inner.MaxY {
							check(t0, y)
						}
					}
				}
			}
		})
	}
}

// TestEdgeCases covers empty inputs.
func TestEdgeCases(t *testing.T) {
	terrain := testTerrain()
	if got := Query(Layer{Seed: 1}, terrain, Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}); got != nil {
		t.Fatalf("empty layer: expected nil, got %d instances", len(got))
	}
	l := Layer{Seed: 1, Species: testSpecies()}
	if got := Query(l, terrain, Rect{MinX: 5, MinY: 5, MaxX: 5, MaxY: 9}); got != nil {
		t.Fatalf("empty window: expected nil, got %d instances", len(got))
	}
	if got := Query(l, terrain, Rect{MinX: 9, MinY: 0, MaxX: 5, MaxY: 9}); got != nil {
		t.Fatalf("inverted window: expected nil, got %d instances", len(got))
	}
}
