// MapLibre style definitions for the vector OSM base map and satellite overlay.
// Extracted from Maps.svelte to keep the component focused on interaction logic.

/**
 * Geometry layers (fill, line) for the vector OSM base map.
 * These render landcover, roads, buildings, water, and boundaries
 * from the locally selected PMTiles archive.
 */
export function vectorBaseLayers() {
  return [
    {
      id: "landcover",
      type: "fill" as const,
      source: "vector-osm",
      "source-layer": "landcover",
      paint: {
        "fill-color": [
          "match",
          ["get", "class"],
          "wood",
          "#c8dfb3",
          "forest",
          "#c8dfb3",
          "grass",
          "#d1e6b0",
          "ice",
          "#e8f0f5",
          "sand",
          "#f2e9c9",
          "#e0ead6",
        ] as any,
        "fill-opacity": 0.6,
      },
    },
    {
      id: "landuse",
      type: "fill" as const,
      source: "vector-osm",
      "source-layer": "landuse",
      paint: {
        "fill-color": [
          "match",
          ["get", "class"],
          "residential",
          "#e6e2da",
          "commercial",
          "#f2dad9",
          "industrial",
          "#eaddf0",
          "cemetery",
          "#c8dfb3",
          "#ebe8e2",
        ] as any,
        "fill-opacity": 0.5,
      },
    },
    {
      id: "park",
      type: "fill" as const,
      source: "vector-osm",
      "source-layer": "park",
      paint: { "fill-color": "#d1e6b0", "fill-opacity": 0.5 },
    },
    {
      id: "water",
      type: "fill" as const,
      source: "vector-osm",
      "source-layer": "water",
      paint: { "fill-color": "#a9cee4" },
    },
    {
      id: "waterway",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "waterway",
      paint: { "line-color": "#a9cee4", "line-width": 1.2 },
    },
    {
      id: "building",
      type: "fill" as const,
      source: "vector-osm",
      "source-layer": "building",
      minzoom: 13,
      paint: { "fill-color": "#d9d0c9", "fill-opacity": 0.8 },
    },
    {
      id: "road-path",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "transportation",
      filter: ["in", ["get", "class"], ["literal", ["path", "track"]]] as any,
      minzoom: 14,
      paint: {
        "line-color": "#c9c2b8",
        "line-width": 0.6,
        "line-dasharray": [2, 1] as any,
      },
    },
    {
      id: "road-minor",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "transportation",
      filter: [
        "in",
        ["get", "class"],
        ["literal", ["minor", "service"]],
      ] as any,
      minzoom: 12,
      paint: {
        "line-color": "#ffffff",
        "line-width": [
          "interpolate",
          ["linear"],
          ["zoom"],
          12,
          0.5,
          18,
          3,
        ] as any,
      },
    },
    {
      id: "road-secondary-tertiary",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "transportation",
      filter: [
        "in",
        ["get", "class"],
        ["literal", ["secondary", "tertiary"]],
      ] as any,
      paint: {
        "line-color": "#f7d488",
        "line-width": [
          "interpolate",
          ["linear"],
          ["zoom"],
          8,
          0.6,
          18,
          5,
        ] as any,
      },
    },
    {
      id: "road-primary",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "transportation",
      filter: ["==", ["get", "class"], "primary"] as any,
      paint: {
        "line-color": "#f3a94e",
        "line-width": [
          "interpolate",
          ["linear"],
          ["zoom"],
          7,
          0.8,
          18,
          6,
        ] as any,
      },
    },
    {
      id: "road-trunk-motorway",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "transportation",
      filter: [
        "in",
        ["get", "class"],
        ["literal", ["trunk", "motorway"]],
      ] as any,
      paint: {
        "line-color": "#e8846b",
        "line-width": [
          "interpolate",
          ["linear"],
          ["zoom"],
          5,
          0.8,
          18,
          8,
        ] as any,
      },
    },
    {
      id: "boundary",
      type: "line" as const,
      source: "vector-osm",
      "source-layer": "boundary",
      filter: ["<=", ["get", "admin_level"], 4] as any,
      paint: {
        "line-color": "#b8a9c9",
        "line-width": 1,
        "line-dasharray": [3, 2] as any,
      },
    },
  ];
}

/**
 * Symbol label layers for road names and place/area names.
 * Kept separate so they can be placed above satellite raster tiles —
 * ensures labels are always visible in both Normal and Satellite modes.
 */
export function vectorLabelLayers() {
  return [
    {
      id: "road-labels",
      type: "symbol" as const,
      source: "vector-osm",
      "source-layer": "transportation_name",
      minzoom: 12,
      layout: {
        "text-field": ["get", "name"] as any,
        "text-font": ["Open Sans Regular"] as any,
        "symbol-placement": "line" as const,
        "text-size": 13,
      },
      paint: {
        "text-color": "#ffffff",
        "text-halo-color": "#1e293b",
        "text-halo-width": 1.5,
      },
    },
    {
      id: "place-labels",
      type: "symbol" as const,
      source: "vector-osm",
      "source-layer": "place",
      filter: ["!=", ["get", "class"], "continent"] as any,
      layout: {
        "text-field": ["get", "name"] as any,
        "text-font": ["Open Sans Regular"] as any,
        "text-size": [
          "interpolate",
          ["linear"],
          ["zoom"],
          4,
          12,
          10,
          16,
          16,
          20,
        ] as any,
        "text-variable-anchor": ["top", "bottom", "left", "right"] as any,
        "text-radial-offset": 0.5,
        "text-justify": "auto" as const,
      },
      paint: {
        "text-color": "#ffffff",
        "text-halo-color": "#1e293b",
        "text-halo-width": 1.5,
      },
    },
  ];
}

/**
 * Builds the full MapLibre style object for the given mode.
 *
 * Normal mode: vector OSM base map only (fully offline with a local PMTiles file).
 * Hybrid mode: vector base + satellite raster overlay (online MapTiler tiles
 * with offline fallback from previously downloaded regions).
 */
export function getStyle(
  mode: "normal" | "hybrid",
  apiKey: string,
  mapFilePath: string,
) {
  const isHybrid = mode === "hybrid";
  const satelliteOnlineUrl = `https://api.maptiler.com/maps/hybrid/{z}/{x}/{y}.jpg?key=${apiKey}`;

  return {
    version: 8 as const,
    // Font glyphs served from the Go backend's embedded Open Sans set
    // (assets/fonts/) so labels render fully offline.
    glyphs: `${window.location.origin}/fonts/{fontstack}/{range}.pbf`,
    sources: {
      "vector-osm": {
        type: "vector" as const,
        // Absolute URL avoids WKWebView path resolution issues with
        // relative URLs for vector tile XHR fetches. Cache-busted with
        // the selected file path so switching .pmtiles archives is
        // recognised as a new source by MapLibre's style diffing.
        tiles: [
          `${window.location.origin}/vtiles/{z}/{x}/{y}.mvt?v=${encodeURIComponent(mapFilePath)}`,
        ],
        minzoom: 0,
        maxzoom: 14,
      },
      "satellite-online": {
        type: "raster" as const,
        tiles: [satelliteOnlineUrl],
        tileSize: 256,
        attribution: "&copy; MapTiler &copy; OpenStreetMap",
      },
      "satellite-offline": {
        type: "raster" as const,
        tiles: [`/tiles/hybrid/{z}/{x}/{y}.png`],
        tileSize: 256,
      },
    },
    layers: [
      {
        id: "background",
        type: "background" as const,
        paint: { "background-color": "#f0f0f0" },
      },
      ...vectorBaseLayers(),
      // Offline satellite drawn first (underneath) as fallback;
      // online layer on top paints over it wherever tiles load.
      {
        id: "satellite-offline-layer",
        type: "raster" as const,
        source: "satellite-offline",
        paint: { "raster-opacity": isHybrid ? 1 : 0 },
      },
      {
        id: "satellite-online-layer",
        type: "raster" as const,
        source: "satellite-online",
        paint: { "raster-opacity": isHybrid ? 1 : 0 },
      },
      // Labels above satellite so road/place names stay legible.
      ...vectorLabelLayers(),
    ],
  };
}