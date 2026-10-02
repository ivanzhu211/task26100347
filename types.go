// Package vegscatter provides deterministic vegetation scattering over a
// procedural terrain. Conceptually there is a single global placement for a
// given (Layer, Terrain) pair; Query merely reveals the slice of it that
// falls inside the requested window.
package vegscatter

import "sort"

// Species describes one plant species in a layer.
type Species struct {
	Name string
	// BaseRadius is the footprint radius (meters) at scale 1.
	BaseRadius float64
	// Weight is the relative selection weight used when drawing species.
	Weight float64
	// MinScale / MaxScale bound the per-instance scale.
	MinScale float64
	MaxScale float64
	// MaxSlope is the maximum terrain slope (degrees) the species tolerates.
	MaxSlope float64
	// MinHeight / MaxHeight bound the elevation (meters) the species tolerates.
	MinHeight float64
	MaxHeight float64
}

// MinRadius returns the minimum footprint radius: BaseRadius * MinScale.
func (s Species) MinRadius() float64 { return s.BaseRadius * s.MinScale }

// MaxRadius returns the maximum footprint radius: BaseRadius * MaxScale.
func (s Species) MaxRadius() float64 { return s.BaseRadius * s.MaxScale }

// Layer is a vegetation layer: a seed plus a set of species.
type Layer struct {
	Seed    uint64
	Species []Species
}

// Terrain is given by two pure functions: height (meters) and slope (degrees).
type Terrain struct {
	Height func(x, y float64) float64
	Slope  func(x, y float64) float64
}

// Rect is an axis-aligned rectangle, closed on the Min side and open on the
// Max side: MinX <= x < MaxX, MinY <= y < MaxY.
type Rect struct {
	MinX, MinY, MaxX, MaxY float64
}

// Contains reports whether the point lies in the rectangle (left-closed,
// right-open).
func (r Rect) Contains(x, y float64) bool {
	return x >= r.MinX && x < r.MaxX && y >= r.MinY && y < r.MaxY
}

// Instance is one placed plant.
type Instance struct {
	// Species indexes into Layer.Species.
	Species int
	X, Y    float64
	Scale   float64
}

// Radius returns the footprint radius of the instance.
func (inst Instance) Radius(l *Layer) float64 {
	return l.Species[inst.Species].BaseRadius * inst.Scale
}

// Query returns all instances of the global placement whose centers fall
// inside window. The result depends only on layer, terrain and window; it is
// independent of call order and of any previous queries.
func Query(layer *Layer, terrain Terrain, window Rect) []Instance {
	p := newPlacer(layer, terrain)
	out := p.collect(window, nil)
	sort.Slice(out, func(i, j int) bool { return instanceLess(out[i], out[j]) })
	return out
}

// instanceLess is a deterministic total order on instances. Coordinates come
// from hashed jitter, so exact ties are practically impossible; the remaining
// fields keep the order total regardless.
func instanceLess(a, b Instance) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	if a.Species != b.Species {
		return a.Species < b.Species
	}
	return a.Scale < b.Scale
}
