# vegscatter

Deterministic vegetation scattering over a procedural terrain (Go, no
dependencies).

Conceptually there is exactly **one global placement** of instances for a
given `(Layer, Terrain)` pair. `Query` merely returns the slice of that
global placement whose centers fall inside a rectangular window:

```go
instances := vegscatter.Query(layer, terrain, vegscatter.Rect{
    MinX: 13.7, MinY: -41.2, MaxX: 73.7, MaxY: 18.8,
})
```

- `Layer` — a `Seed` plus a list of `Species`. Each species has a base
  occupancy radius, a selection weight, a scale range, a maximum slope
  (degrees) and an elevation range (meters).
- `Terrain` — two pure functions `Height(x, y)` (m) and `Slope(x, y)`
  (degrees).
- `Instance` — species index, center, scale. Occupancy circle radius =
  `BaseRadius * Scale`; slope and elevation at the center are always legal
  for the species. Windows are half-open: `MinX <= x < MaxX`.

## Guarantees

The global placement satisfies, across any window decomposition and any
internal blocking:

- **Non-overlap** — no two occupancy circles intersect.
- **Saturation** — wherever a species is legal, there is no empty spot
  where a disc of `1.5 x` its minimum occupancy radius would fit without
  overlapping an instance. (Assuming reasonably smooth terrain: legality
  at a point must imply legality within ~0.5 m around it.)
- **Consistency** — any two windows agree bitwise on their overlap; the
  result depends only on `(layer, terrain, window)`, never on call
  history.

## How it works

1. The plane is partitioned into a global grid of square **blocks** of
   side `2 * maxRadius` (max over species of `BaseRadius * MaxScale`).
2. Blocks are colored with a **3 x 3 phase pattern**; same-phase blocks
   are too far apart to ever interact. Phases are resolved in one fixed
   global order, so acceptance only ever reads instances accepted by
   earlier phases — the outcome is window-independent.
3. Inside a block, the points of a global **sub-lattice** (spacing
   `minRadius / 1.5`) are resolved sequentially. Each lattice point first
   tries a weighted species pick with a random scale and a small jitter,
   then retries every species at its minimum scale exactly on the lattice
   point. The retries are what makes saturation provable: any legal empty
   `1.5 x minRadius` disc contains a lattice point whose min-scale retry
   could not have been rejected.
4. All randomness comes from a splitmix64-style hash of
   `(seed, lattice i, lattice j, attempt)`, so everything is deterministic
   and reproducible.
5. `Query` evaluates the blocks covering the window plus a fixed
   8-block dependency halo, then clips the result to the window.

## Tests

```
go test ./...
```

- `TestTiledUnionMatchesBigWindow` — a big window is cut into a 3x3
  tiling along deliberately arbitrary lines; the union of tile results
  must equal the big-window result bitwise.
- `TestDeterministic` — repeated queries are identical.
- `TestNoOverlap` — pairwise circle check over the tiled union.
- `TestSaturation` — dense sampling (plus extra sampling straddling the
  seam lines) verifies no legal `1.5 x minRadius` spot is left empty.
- `TestEdgeCases` — empty layer / degenerate windows.
