package vegscatter

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

// testTerrain returns a smooth terrain: elevation wanders in [-60, 60] and
// slope in roughly [3, 27] degrees, both varying on hundred-meter scales.
func testTerrain() Terrain {
	return Terrain{
		Height: func(x, y float64) float64 {
			return 40*math.Sin(x*0.008)*math.Cos(y*0.011) + 0.02*x
		},
		Slope: func(x, y float64) float64 {
			return 15 + 12*math.Sin(x*0.013+1.7)*math.Sin(y*0.009-0.4)
		},
	}
}

func testLayer() *Layer {
	return &Layer{
		Seed: 42,
		Species: []Species{
			{Name: "grass", BaseRadius: 0.8, Weight: 3, MinScale: 1.0, MaxScale: 1.6, MaxSlope: 40, MinHeight: -100, MaxHeight: 100},
			{Name: "bush", BaseRadius: 1.5, Weight: 2, MinScale: 0.9, MaxScale: 1.4, MaxSlope: 22, MinHeight: -25, MaxHeight: 35},
			{Name: "tree", BaseRadius: 2.2, Weight: 1, MinScale: 0.8, MaxScale: 1.3, MaxSlope: 15, MinHeight: -15, MaxHeight: 25},
		},
	}
}

func testWindow() Rect {
	return Rect{MinX: 213.73, MinY: -418.91, MaxX: 213.73 + 72, MaxY: -418.91 + 60}
}

// splitRect cuts r into a grid of sub-rectangles along deliberately
// non-uniform, non-integer cut lines.
func splitRect(r Rect, nx, ny int) []Rect {
	xs := irregularCuts(r.MinX, r.MaxX, nx)
	ys := irregularCuts(r.MinY, r.MaxY, ny)
	var out []Rect
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			out = append(out, Rect{MinX: xs[i], MaxX: xs[i+1], MinY: ys[j], MaxY: ys[j+1]})
		}
	}
	return out
}

func irregularCuts(lo, hi float64, n int) []float64 {
	cuts := []float64{lo}
	for i := 1; i < n; i++ {
		// Deterministic but irregular fractions, all strictly interior.
		frac := float64(i)/float64(n) + 0.013*math.Sin(float64(i)*2.39)
		cuts = append(cuts, lo+(hi-lo)*frac)
	}
	return append(cuts, hi)
}

// instanceKey encodes an instance with exact float bits for bit-identical
// comparison.
func instanceKey(l *Layer, inst Instance) string {
	return fmt.Sprintf("%d/%x/%x/%x", inst.Species,
		math.Float64bits(inst.X), math.Float64bits(inst.Y), math.Float64bits(inst.Scale))
}

func sortedKeys(l *Layer, insts []Instance) []string {
	keys := make([]string, len(insts))
	for i, inst := range insts {
		keys[i] = instanceKey(l, inst)
	}
	sort.Strings(keys)
	return keys
}

// TestPartitionConsistency is the main scenario test: a big window is cut
// into small windows along arbitrary cut lines; the union of the small query
// results must equal the big query result bit-for-bit, be non-overlapping,
// and stay saturated near the seams.
func TestPartitionConsistency(t *testing.T) {
	layer := testLayer()
	terrain := testTerrain()
	big := testWindow()

	bigResult := Query(layer, terrain, big)
	if len(bigResult) == 0 {
		t.Fatal("big window query returned no instances")
	}

	parts := splitRect(big, 5, 4)
	var union []Instance
	// Query parts in a scrambled order to prove order-independence.
	order := []int{7, 12, 0, 19, 3, 15, 8, 1, 17, 10, 5, 13, 2, 18, 9, 4, 16, 11, 6, 14}
	for _, idx := range order {
		union = append(union, Query(layer, terrain, parts[idx])...)
	}

	gotKeys := sortedKeys(layer, union)
	wantKeys := sortedKeys(layer, bigResult)
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("union has %d instances, big window has %d", len(gotKeys), len(wantKeys))
	}
	for i := range gotKeys {
		if gotKeys[i] != wantKeys[i] {
			t.Fatalf("instance %d differs between union and big window:\nunion: %s\nbig:   %s", i, gotKeys[i], wantKeys[i])
		}
	}

	checkNonOverlap(t, layer, union)
	checkSeamSaturation(t, layer, terrain, big, parts, union)
}

// checkNonOverlap verifies that no two footprint circles intersect.
func checkNonOverlap(t *testing.T, l *Layer, insts []Instance) {
	t.Helper()
	for i := 0; i < len(insts); i++ {
		for j := i + 1; j < len(insts); j++ {
			a, b := insts[i], insts[j]
			r := a.Radius(l) + b.Radius(l)
			dx, dy := a.X-b.X, a.Y-b.Y
			if dx*dx+dy*dy < r*r-1e-9 {
				t.Fatalf("instances %d (%s at %.3f,%.3f r=%.3f) and %d (%s at %.3f,%.3f r=%.3f) overlap",
					i, l.Species[a.Species].Name, a.X, a.Y, a.Radius(l),
					j, l.Species[b.Species].Name, b.X, b.Y, b.Radius(l))
			}
		}
	}
}

// legalAt reports whether species sp may sit at (x, y).
func legalAt(t Terrain, sp Species, x, y float64) bool {
	if t.Slope(x, y) > sp.MaxSlope {
		return false
	}
	h := t.Height(x, y)
	return h >= sp.MinHeight && h <= sp.MaxHeight
}

// stablyLegal reports whether species sp is legal at p and on a ring of
// radius 0.6*minR around it, i.e. the neighborhood is solidly legal.
func stablyLegal(t Terrain, sp Species, x, y float64) bool {
	if !legalAt(t, sp, x, y) {
		return false
	}
	ring := 0.6 * sp.MinRadius()
	for k := 0; k < 8; k++ {
		a := float64(k) * math.Pi / 4
		if !legalAt(t, sp, x+ring*math.Cos(a), y+ring*math.Sin(a)) {
			return false
		}
	}
	return true
}

// checkSaturationAt asserts that no legal empty spot remains: for every
// sample point where a species is stably legal, some instance must overlap
// the disc of radius 1.5*minR around the point.
func checkSaturationAt(t *testing.T, l *Layer, terrain Terrain, insts []Instance, px, py float64) {
	t.Helper()
	for si, sp := range l.Species {
		if !stablyLegal(terrain, sp, px, py) {
			continue
		}
		reach := 1.5 * sp.MinRadius()
		covered := false
		for _, inst := range insts {
			r := reach + inst.Radius(l)
			dx, dy := px-inst.X, py-inst.Y
			if dx*dx+dy*dy < r*r {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("saturation violated at (%.3f, %.3f): species %s (min radius %.3f) fits a %.3f-radius disc with no instance nearby",
				px, py, l.Species[si].Name, sp.MinRadius(), reach)
		}
	}
}

// checkSeamSaturation samples densely along the internal cut lines of the
// partition, plus a coarse grid over the whole window interior.
func checkSeamSaturation(t *testing.T, l *Layer, terrain Terrain, big Rect, parts []Rect, insts []Instance) {
	t.Helper()
	// Collect internal cut lines from the partition.
	xCuts := map[int64]float64{}
	yCuts := map[int64]float64{}
	key := func(v float64) int64 { return int64(math.Round(v * 1e6)) }
	for _, p := range parts {
		if p.MinX > big.MinX {
			xCuts[key(p.MinX)] = p.MinX
		}
		if p.MinY > big.MinY {
			yCuts[key(p.MinY)] = p.MinY
		}
	}
	// Instances just outside the big window are invisible to the union, so
	// only sample points whose whole test disc lies inside the window.
	maxR := 0.0
	for _, sp := range l.Species {
		if sp.MaxRadius() > maxR {
			maxR = sp.MaxRadius()
		}
	}
	margin := 1.5*l.Species[0].MinRadius() + maxR + 0.1
	for _, sp := range l.Species {
		if m := 1.5*sp.MinRadius() + maxR + 0.1; m > margin {
			margin = m
		}
	}
	inside := func(x, y float64) bool {
		return x > big.MinX+margin && x < big.MaxX-margin && y > big.MinY+margin && y < big.MaxY-margin
	}
	const step = 0.4
	for _, xc := range xCuts {
		for y := big.MinY + margin; y < big.MaxY-margin; y += step {
			if inside(xc, y) {
				checkSaturationAt(t, l, terrain, insts, xc, y)
			}
		}
	}
	for _, yc := range yCuts {
		for x := big.MinX + margin; x < big.MaxX-margin; x += step {
			if inside(x, yc) {
				checkSaturationAt(t, l, terrain, insts, x, yc)
			}
		}
	}
	for x := big.MinX + margin; x < big.MaxX-margin; x += 3.1 {
		for y := big.MinY + margin; y < big.MaxY-margin; y += 2.7 {
			checkSaturationAt(t, l, terrain, insts, x, y)
		}
	}
}

// TestDeterminismAndOrderIndependence repeats queries in different orders and
// expects bit-identical results.
func TestDeterminismAndOrderIndependence(t *testing.T) {
	layer := testLayer()
	terrain := testTerrain()
	big := testWindow()
	parts := splitRect(big, 3, 3)

	first := sortedKeys(layer, Query(layer, terrain, big))
	// Interleave part queries and the big query; results must not change.
	var union []Instance
	for i, p := range parts {
		union = append(union, Query(layer, terrain, p)...)
		if i == 4 {
			again := sortedKeys(layer, Query(layer, terrain, big))
			if fmt.Sprint(again) != fmt.Sprint(first) {
				t.Fatal("repeated big-window query returned different results")
			}
		}
	}
	if fmt.Sprint(sortedKeys(layer, union)) != fmt.Sprint(first) {
		t.Fatal("union of parts differs from big-window result")
	}
}

// TestEdgeCases covers degenerate inputs.
func TestEdgeCases(t *testing.T) {
	terrain := testTerrain()
	empty := &Layer{Seed: 1}
	if got := Query(empty, terrain, testWindow()); len(got) != 0 {
		t.Fatalf("empty layer returned %d instances", len(got))
	}
	zeroWeight := &Layer{
		Seed: 2,
		Species: []Species{
			{Name: "never", BaseRadius: 1, Weight: 0, MinScale: 1, MaxScale: 1, MaxSlope: 90, MinHeight: -1e9, MaxHeight: 1e9},
		},
	}
	if got := Query(zeroWeight, terrain, testWindow()); len(got) != 0 {
		t.Fatalf("zero-weight species returned %d instances", len(got))
	}
}
