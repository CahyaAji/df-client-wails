# Converting OSM `.pbf` to `.pmtiles`

This guide covers producing a vector `.pmtiles` file (from a Geofabrik `.osm.pbf`
extract) that can be loaded into this app as the offline OSM base map via the
**"Select Map File"** toolbar button in [`Maps.svelte`](../frontend/src/lib/component/Maps.svelte).

## Prerequisites

- **Java** — required to run Planetiler (a single executable JAR). Check with:
  ```bash
  java -version
  ```
- **Go** — required to install the `go-pmtiles` CLI (only needed for the
  MBTiles → PMTiles fallback conversion in Step 2b). Check with:
  ```bash
  go version
  ```

## Step 1 — Download Planetiler

Planetiler is a single JAR, no separate installation needed beyond Java:

```bash
curl -L -o planetiler.jar https://github.com/onthegomap/planetiler/releases/latest/download/planetiler.jar
```

## Step 2 — Run Planetiler on your `.osm.pbf`

### Option A — Direct PMTiles output (preferred)

Recent Planetiler versions can write directly to PMTiles format in one step:

```bash
java -Xmx4g -jar planetiler.jar \
  --download \
  --force \
  --mmap-temp=false \
  --compress-temp=true \
  --osm-path=java-island.osm.pbf \
  --output=java.pmtiles
```

- `-Xmx4g` caps the Java heap at 4GB — increase for larger extracts (e.g. a
  whole province/country), decrease for small city-sized areas.
- This produces the standard **OpenMapTiles vector schema** (layers such as
  `water`, `landcover`, `landuse`, `park`, `building`, `transportation`,
  `boundary`, etc.) — exactly the layer names referenced by
  `vectorBaseLayers()` in [`Maps.svelte`](../frontend/src/lib/component/Maps.svelte).

### Option B — Via MBTiles (fallback for older Planetiler versions)

If your Planetiler build doesn't support direct `.pmtiles` output, produce
standard MBTiles first, then convert:

```bash
# Step 2a — produce standard MBTiles
java -Xmx4g -jar planetiler.jar \
  --osm-path=your-area.osm.pbf \
  --output=your-area.mbtiles
```

## Step 2b — Convert MBTiles → PMTiles (only needed for Option B)

Install the `go-pmtiles` CLI:

```bash
go install github.com/protomaps/go-pmtiles@v1.31.2
```

This installs a binary named `go-pmtiles` into `$(go env GOPATH)/bin`. Then run:

```bash
$(go env GOPATH)/bin/go-pmtiles convert your-area.mbtiles your-area.pmtiles
```

The `go-pmtiles` CLI supports several other useful subcommands:

```
Commands:
  show <path>              Inspect a local or remote archive
  tile <path> <z> <x> <y>  Fetch one tile from a local or remote archive
  cluster <input>          Cluster an unclustered local archive
  edit <input>             Edit JSON metadata or parts of the header
  extract <input> <output> Create an archive for a subset of zoom/region
  merge <inputs...> <out>  Merge multiple disjoint archives into one
  convert <input> <output> Convert an MBTiles database to PMTiles
  verify <input>           Verify the correctness of an archive structure
  serve <path>              Run an HTTP proxy server for Z/X/Y tiles
  upload --bucket=... ...   Upload a local archive to remote storage
```

## Step 3 — Verify the resulting file

```bash
$(go env GOPATH)/bin/go-pmtiles verify your-area.pmtiles
$(go env GOPATH)/bin/go-pmtiles show your-area.pmtiles
```

`show` prints the header/metadata (min/max zoom, bounds, tile type) so you can
confirm it's a valid `Mvt` (vector) archive before loading it into the app.

## Step 4 — Load it into the app

1. Launch the app.
2. Click the **"Select Map File"** toolbar button.
3. Pick `your-area.pmtiles` in the native file dialog.

The Go backend opens it via `loadVectorMap()` in [`main.go`](../main.go) and
immediately starts serving tiles at `/vtiles/{z}/{x}/{y}.mvt`. The map style
reloads automatically to show the new base layer, and the choice is persisted
to `config.json` so it's restored automatically on the next launch.

## Quick reference

| Step | Command |
|---|---|
| Get Planetiler | `curl -L -o planetiler.jar https://github.com/onthegomap/planetiler/releases/latest/download/planetiler.jar` |
| PBF → PMTiles (direct) | `java -jar planetiler.jar --osm-path=area.osm.pbf --output=area.pmtiles` |
| PBF → MBTiles (fallback) | `java -jar planetiler.jar --osm-path=area.osm.pbf --output=area.mbtiles` |
| Install go-pmtiles CLI | `go install github.com/protomaps/go-pmtiles@v1.31.2` |
| MBTiles → PMTiles | `go-pmtiles convert area.mbtiles area.pmtiles` |
| Verify | `go-pmtiles verify area.pmtiles` |
| Inspect | `go-pmtiles show area.pmtiles` |

## Tips

- For a first test, use a **small extract** (a city/district, not a whole
  province) so both the Planetiler build and the app-side testing loop are
  fast to iterate on.
- Text labels (road names, place names, POI names) are **not yet rendered**
  by the app's vector style — only geometry (roads, water, buildings, land
  use) is drawn. Rendering labels offline would require bundling local font
  glyphs, which is a possible future enhancement.
- The satellite/hybrid view is unaffected by this workflow — it still uses
  the existing raster tile download/cache pipeline (MapTiler API +
  `map_data.mbtiles`), since satellite imagery cannot be represented as
  vector data.
