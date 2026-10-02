// Package vegscatter provides deterministic vegetation scattering over a
// procedural terrain. Conceptually there is a single global placement of
// instances for a given (Layer, Terrain) pair; Query merely returns the
// slice of that global placement whose centers fall inside a window.
package vegscatter

// Species describes one plant species in a layer.
type Species struct {
	Name       string  // informational only
	BaseRadius float64 // occupancy radius (meters) at scale 1.0
	Weight     float64 // relative selection weight
	MinScale   float64 // minimum random scale
	MaxScale   float64 // maximum random scale
	MaxSlope   float64 // maximum terrain slope (degrees) at the center
	MinElev    float64 // minimum elevation (meters) at the center
	MaxElev    float64 // maximum elevation (meters) at the center
}

// MinRadius returns the smallest occupancy radius this species can have.
func (s Species) MinRadius() float64 { return s.BaseRadius * s.MinScale }

// MaxRadius returns the largest occupancy radius this species can have.
func (s Species) MaxRadius() float64 { return s.BaseRadius * s.MaxScale }

// Layer is a vegetation layer: a seed plus a set of species.
type Layer struct {
	Seed    uint64
	Species []Species
}

// Terrain is given by two pure functions.
type Terrain struct {
	Height func(x, y float64) float64 // meters
	Slope  func(x, y float64) float64 // degrees
}

// Rect is an axis-aligned window, left/bottom closed, right/top open:
// MinX <= x < MaxX and MinY <= y < MaxY.
type Rect struct {
	MinX, MinY, MaxX, MaxY float64
}

// Contains reports whether the point lies in the rectangle (half-open).
func (r Rect) Contains(x, y float64) bool {
	return x >= r.MinX && x < r.MaxX && y >= r.MinY && y < r.MaxY
}

// Instance is a single placed plant.
type Instance struct {
	Species int     // index into Layer.Species
	X, Y    float64 // center
	Scale   float64 // random scale in [MinScale, MaxScale]
}

// Radius returns the occupancy radius of the instance.
func (in Instance) Radius(l Layer) float64 {
	return l.Species[in.Species].BaseRadius * in.Scale
}
