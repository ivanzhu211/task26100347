# vegscatter

Deterministic vegetation scattering for procedural terrains.

Given a vegetation **layer** (a seed plus a set of species) and a **terrain**
(two pure functions `Height(x, y)` in meters and `Slope(x, y)` in degrees),
`Query` returns, for any axis-aligned window, the instances of the single
global placement whose centers fall inside that window.

```go
layer := &vegscatter.Layer{
    Seed: 42,
    Species: []vegscatter.Species{
        {Name: "grass", BaseRadius: 0.8, Weight: 3, MinScale: 1.0, MaxScale: 1.6,
         MaxSlope: 40, MinHeight: -100, MaxHeight: 100},
        // ...
    },
}
terrain := vegscatter.Terrain{Height: heightFn, Slope: slopeFn}
insts := vegscatter.Query(layer, terrain, vegscatter.Rect{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100})
```

An instance is a species index, a center and a scale; its footprint circle
has radius `BaseRadius * scale`, and the slope/elevation at its center are
legal for its species.

## Guarantees

The global placement defined by a `(Layer, Terrain)` pair satisfies:

- **Non-overlap.** No two footprint circles intersect — across windows and
  across any internal tiling boundaries.
- **Saturation.** Wherever a species is legal, no empty spot remains where a
  disc of `1.5x` its minimum footprint radius would fit without overlapping
  any instance. (This holds exactly where terrain legality is stable within
  half a minimum radius; the filler grid that backs the guarantee has a
  worst-case gap of `0.35*sqrt(2) < 0.5` minimum radii.)
- **Consistency.** Any two windows agree exactly on their overlap. Results
  depend only on the layer, terrain and window — never on call order or
  previously queried windows. All randomness derives from integer hashing of
  the layer seed and grid coordinates, so results are bit-identical across
  runs and platforms.

## How it works

Candidates are generated lazily from two world-fixed grids, so every query
sees the same candidate stream:

- **Main tier**: one candidate per `2*maxRadius` cell, with a weight-drawn
  species and a random scale in the species' range.
- **Filler tier**: per species, a dense grid (`0.35 * minRadius` spacing) of
  minimum-scale candidates that back the saturation guarantee.

A candidate is kept iff it is valid (terrain-legal at its center) and no
*accepted* candidate that outranks it (deterministic priority, main tier
always outranks filler tier) overlaps its footprint. Acceptance is therefore
a pure function of the candidate identity; `Query` resolves it lazily with
per-call memoization and collects the accepted candidates whose centers fall
inside the window. Priorities strictly increase along every dependency
chain, so resolution terminates and is window-independent.

## Tests

`go test ./...` covers the required scenario: a large window is cut into
small windows along deliberately irregular cut lines; the union of the small
results must equal the big-window result bit-for-bit, the union is checked
for overlaps, and saturation is verified along the seams and on a coarse
interior grid. Additional tests cover determinism/order-independence and
degenerate layers.
