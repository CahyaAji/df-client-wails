<script lang="ts">
    import maplibregl from "maplibre-gl";
    import type * as GeoJSON from "geojson";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { onMount, onDestroy } from "svelte";
    import { locationStore } from "../store/locationStore.svelte.js";
    import { dfStore } from "../store/dfStore.svelte.js";
    import { signalState } from "../store/signalState.svelte.js";

    import {
        ClearDownloads,
        DownloadRegion,
        ListBookmarks,
        SelectVectorMapFile,
        GetVectorMapPath,
    } from "../../../wailsjs/go/main/App";
    import { EventsOn } from "../../../wailsjs/runtime/runtime";
    import { configStore } from "../store/configStore.svelte.js";
    import { compassStore } from "../store/compassStore.svelte.js";
    import { vectorBaseLayers, vectorLabelLayers, getStyle } from "./map-styles";
  // Bookmarks state
  let bookmarks: Array<{
    id: number;
    title: string;
    style: string;
    min_zoom: number;
    max_zoom: number;
    north: number;
    south: number;
    east: number;
    west: number;
    center_lat: number;
    center_lng: number;
  }> = $state([]);

  async function fetchBookmarks() {
    try {
      bookmarks = (await ListBookmarks()) ?? [];
    } catch (e) {
      console.error("Failed to fetch bookmarks", e);
    }
  }

  function goToBookmark(b: (typeof bookmarks)[number]) {
    if (map) {
      map.setCenter([b.center_lng, b.center_lat]);
      map.setZoom(b.min_zoom);
      switchStyle(b.style === "hybrid" ? "hybrid" : "normal");
    }
  }

  function findMyLocation() {
    const { latitude, longitude } = locationStore.data;
    if (!map) return;
    if (latitude !== null && longitude !== null) {
      if (!locationMarker) {
        locationMarker = new maplibregl.Marker({
          element: createLocationElement(),
          anchor: "center",
        })
          .setLngLat([longitude, latitude])
          .addTo(map);
      }
      map.flyTo({
        center: [longitude, latitude],
        zoom: 12,
        essential: true,
      });
    } else {
      console.log("Current location not available");
    }
  }

  function resetMapView() {
    if (!map) return;
    map.easeTo({
      bearing: 0,
      pitch: 0,
      duration: 300,
      essential: true,
    });
  }

  let mapContainer: HTMLElement;
  let map: maplibregl.Map;

  // Svelte 5 State
  let isDownloading = $state(false);
  let downloadStatus = $state("Ready");
  let currentMode = $state<"normal" | "hybrid">("normal"); // 'normal' or 'hybrid'
  let showDownloadPanel = $state(false);
  let downloadTab = $state<"download" | "bookmarks">("bookmarks");
  let completionNotice = $state("");
  let downloadTitle = $state("");
  let noticeTimer: ReturnType<typeof setTimeout> | null = null;
  let unsubscribeDownloadEvents: (() => void) | null = null;
  let mapResizeObserver: ResizeObserver | null = null;
  let currentZoom = $state(14);
  let isClearing = $state(false);
  let downloadLocked = $state(false);
  let mapFilePath = $state("");
  let isLoadingMap = $state(false);

  let locationMarker: maplibregl.Marker | null = null;

  // --- User Markers ---
  type MarkerDirection = {
    id: number;
    angle: number;
    color: string;
  };
  type UserMarker = {
    id: number;
    name: string;
    lat: number;
    lng: number;
    directions: MarkerDirection[];
    mapMarker: maplibregl.Marker;
  };

  // Serialisable shape — excludes the live maplibregl.Marker instance
  type StoredMarker = Omit<UserMarker, "mapMarker">;
  const MARKERS_STORAGE_KEY = "df_client_user_markers";
  const DEFAULT_DIRECTION_COLOR = "#2563eb";

  function saveMarkersToStorage() {
    const data: StoredMarker[] = userMarkers.map(
      ({ id, name, lat, lng, directions }) => ({
        id,
        name,
        lat,
        lng,
        directions,
      }),
    );
    localStorage.setItem(MARKERS_STORAGE_KEY, JSON.stringify(data));
  }

  function restoreMarkersFromStorage() {
    if (!map) return;
    try {
      const raw = localStorage.getItem(MARKERS_STORAGE_KEY);
      if (!raw) return;
      const data: StoredMarker[] = JSON.parse(raw);
      if (!Array.isArray(data)) return;
      for (const d of data) {
        const el = createCustomMarkerElement(d.name);
        const mapMarker = new maplibregl.Marker({
          element: el,
          anchor: "center",
        })
          .setLngLat([d.lng, d.lat])
          .addTo(map);
        userMarkers = [...userMarkers, { ...d, mapMarker }];
        for (const dir of d.directions) {
          addDirectionLine(d.id, dir.id, d.lat, d.lng, dir.angle, dir.color);
        }
        if (d.id >= markerIdCounter) markerIdCounter = d.id + 1;
        const maxDirId = d.directions.reduce(
          (max, dir) => Math.max(max, dir.id),
          0,
        );
        if (maxDirId >= directionIdCounter) directionIdCounter = maxDirId + 1;
      }
    } catch {
      console.warn("Failed to restore markers from localStorage");
    }
  }

  let userMarkers: UserMarker[] = $state([]);
  let markerIdCounter = 0;
  let directionIdCounter = 0;
  let showMarkerPanel = $state(false);
  let showMarkerBottomPanel = $state(false);
  let showAddMarkerForm = $state(false);
  let pinPointMode = $state(false);
  let newMarkerName = $state("");
  let newMarkerLat = $state("");
  let newMarkerLng = $state("");
  let newMarkerDirections = $state<MarkerDirection[]>([]);
  let newDirectionAngle = $state("");
  let newDirectionColor = $state(DEFAULT_DIRECTION_COLOR);
  let mapClickHandler: ((e: maplibregl.MapMouseEvent) => void) | null = null;
  let pinPointTempMarker: maplibregl.Marker | null = null;
  let editingMarkerId = $state<number | null>(null);
  // --- End User Markers ---

  // --- DF Heading Line ---

  function destinationPoint(
    lat: number,
    lng: number,
    bearingDeg: number,
    distanceKm: number,
  ): [number, number] {
    const R = 6371;
    const d = distanceKm / R;
    const θ = (bearingDeg * Math.PI) / 180;
    const φ1 = (lat * Math.PI) / 180;
    const λ1 = (lng * Math.PI) / 180;
    const φ2 = Math.asin(
      Math.sin(φ1) * Math.cos(d) + Math.cos(φ1) * Math.sin(d) * Math.cos(θ),
    );
    const λ2 =
      λ1 +
      Math.atan2(
        Math.sin(θ) * Math.sin(d) * Math.cos(φ1),
        Math.cos(d) - Math.sin(φ1) * Math.sin(φ2),
      );
    return [(λ2 * 180) / Math.PI, (φ2 * 180) / Math.PI];
  }

  function updateDFLine() {
    if (!map) return;
    if (!locationMarker) return;

    // Default to null if stores or data are not ready
    const { latitude, longitude } = locationStore.data ?? {};
    const dfHeading = dfStore.data?.heading ?? null;
    const compassHeading = compassStore.data ?? null;
    const compassOffset = signalState.compassOffset || 0;

    // Calculate effective heading only if all components are valid numbers
    const heading =
      dfHeading !== null && compassHeading !== null
        ? (360 + dfHeading + compassHeading + compassOffset) % 360
        : null;

    const hasData = latitude != null && longitude != null && heading !== null;

    const geojson: GeoJSON.FeatureCollection = hasData
      ? {
          type: "FeatureCollection",
          features: [
            {
              type: "Feature",
              geometry: {
                type: "LineString",
                coordinates: [
                  [longitude!, latitude!],
                  destinationPoint(latitude!, longitude!, heading!, 10),
                ],
              },
              properties: {},
            },
          ],
        }
      : { type: "FeatureCollection", features: [] }; // Empty features if no data

    // Always update the source
    const source = map.getSource("df-line") as maplibregl.GeoJSONSource;
    if (source) {
      source.setData(geojson);
    } else {
      // If source doesn't exist, create it (happens on first run)
      map.addSource("df-line", { type: "geojson", data: geojson });
      map.addLayer({
        id: "df-line-layer",
        type: "line",
        source: "df-line",
        paint: {
          "line-color": "#2563eb",
          "line-width": 3,
        },
      });
    }
  }

  // Throttle timer so updateDFLine() runs at most once per second,
  // preventing rapid-fire redraws when dfStore mutates multiple $state
  // fields within a single poll cycle.
  let _dfLineThrottleTimer: ReturnType<typeof setTimeout> | null = null;
  const DF_LINE_THROTTLE_MS = 1000;

  $effect(() => {
    // Re-run whenever location or DF heading changes
    locationStore.data.latitude;
    locationStore.data.longitude;
    dfStore.data;

    if (_dfLineThrottleTimer) return; // throttle window still open

    updateDFLine();
    _dfLineThrottleTimer = setTimeout(() => {
      _dfLineThrottleTimer = null;
    }, DF_LINE_THROTTLE_MS);
  });

  // --- End DF Heading Line ---

  function createLocationElement(): HTMLElement {
    const wrapper = document.createElement("div");
    wrapper.className = "my-location-marker";

    const pulse = document.createElement("div");
    pulse.className = "my-location-pulse";

    const dot = document.createElement("div");
    dot.className = "my-location-dot";

    wrapper.appendChild(pulse);
    wrapper.appendChild(dot);
    return wrapper;
  }

  $effect(() => {
    const { latitude, longitude } = locationStore.data;
    if (!map) return;
    if (latitude !== null && longitude !== null) {
      if (!locationMarker) {
        locationMarker = new maplibregl.Marker({
          element: createLocationElement(),
          anchor: "center",
        })
          .setLngLat([longitude, latitude])
          .addTo(map);
      } else {
        locationMarker.setLngLat([longitude, latitude]);
      }
    } else {
      if (locationMarker) {
        locationMarker.remove();
        locationMarker = null;
      }
    }
  });

  type Bounds = {
    north: number;
    south: number;
    east: number;
    west: number;
  };

  type SelectionPixels = {
    left: number;
    top: number;
    width: number;
    height: number;
  };

  type DownloadEvent = {
    bookmarkId: number;
    title?: string;
    status?: string;
    message?: string;
  };

  let selectMode = $state(false);
  const MIN_ZOOM_LIMIT = 8;
  const MAX_ZOOM_LIMIT = 18;

  let customMinZoom = $state(MIN_ZOOM_LIMIT);
  let customMaxZoom = $state(16);
  let selectionPixels: SelectionPixels | null = $state(null);
  let selectionBounds: Bounds | null = $state(null);

  let isDrawingSelection = false;
  let dragStartPx: { x: number; y: number } | null = null;
  let dragStartLngLat: maplibregl.LngLat | null = null;
  let activePointerId: number | null = null;

  const clamp = (value: number, min: number, max: number) =>
    Math.max(min, Math.min(max, value));

  function normalizeBounds(a: maplibregl.LngLat, b: maplibregl.LngLat): Bounds {
    return {
      north: Math.max(a.lat, b.lat),
      south: Math.min(a.lat, b.lat),
      east: Math.max(a.lng, b.lng),
      west: Math.min(a.lng, b.lng),
    };
  }

  const apiKey = $derived(configStore.mapKey);

  // Base vector layers rendered from the locally selected PMTiles archive
  // (served by the Go backend at /vtiles/{z}/{x}/{y}.mvt). Uses the
  // standard OpenMapTiles layer names produced by Planetiler. Text labels
  // are intentionally omitted for now since rendering them offline would
  // require bundling local font glyphs — geometry (roads, water,
  // buildings, land use) renders fully offline without that dependency.
  function switchStyle(mode: "normal" | "hybrid") {
    currentMode = mode;
    if (mode === "normal") {
      showDownloadPanel = false;
      cancelSelection();
    }
    if (map) {
      map.setStyle(getStyle(mode, apiKey, mapFilePath));
    }
  }

  function disableMapInteractions() {
    if (!map) return;
    map.dragPan.disable();
    map.doubleClickZoom.disable();
    map.scrollZoom.disable();
    map.touchZoomRotate.disable();
    map.boxZoom.disable();
  }

  function enableMapInteractions() {
    if (!map) return;
    map.dragPan.enable();
    map.doubleClickZoom.enable();
    map.scrollZoom.enable();
    map.touchZoomRotate.enable();
    map.boxZoom.enable();
  }

  function resetSelectionDrawing() {
    isDrawingSelection = false;
    dragStartPx = null;
    dragStartLngLat = null;
    if (mapContainer && activePointerId !== null) {
      mapContainer.releasePointerCapture(activePointerId);
    }
    activePointerId = null;
  }

  // Debounce timestamps — prevent double-firing on Windows touchscreens
  // where both pointer/touch events and synthesized mouse clicks are dispatched.
  let _lastDownloadToggleMs = 0;
  let _lastMarkerToggleMs = 0;

  function toggleDownloadPanel(tab: "download" | "bookmarks" = "download") {
    const now = Date.now();
    if (now - _lastDownloadToggleMs < 350) return;
    _lastDownloadToggleMs = now;
    if (showDownloadPanel && downloadTab === tab) {
      // clicking the same tab's trigger closes the panel
      showDownloadPanel = false;
      cancelSelection();
    } else {
      showDownloadPanel = true;
      downloadTab = tab;
      if (tab === "download") {
        syncMinZoomWithCurrent();
        // Offline downloads only ever apply to satellite imagery —
        // switch to hybrid mode so the user is downloading what
        // they expect.
        switchStyle("hybrid");
      }
    }
  }

  function toggleMarkerBottomPanel() {
    const now = Date.now();
    if (now - _lastMarkerToggleMs < 350) return;
    _lastMarkerToggleMs = now;
    showMarkerBottomPanel = !showMarkerBottomPanel;
    if (showMarkerBottomPanel) {
      // auto-open the marker list so it's visible without an extra click
      showMarkerPanel = true;
    }
  }

  function beginSelection() {
    if (!map) return;
    selectionPixels = null;
    selectionBounds = null;
    isDrawingSelection = false;
    dragStartPx = null;
    dragStartLngLat = null;
    selectMode = true;
    disableMapInteractions();
  }

  function cancelSelection() {
    selectMode = false;
    selectionPixels = null;
    selectionBounds = null;
    resetSelectionDrawing();
    enableMapInteractions();
  }

  function finalizeSelection() {
    selectMode = false;
    resetSelectionDrawing();
  }

  function showCompletion(message: string) {
    completionNotice = message;
    if (noticeTimer) {
      clearTimeout(noticeTimer);
    }
    noticeTimer = window.setTimeout(() => {
      completionNotice = "";
      noticeTimer = null;
    }, 4000);
  }

  function zoomInputsValid() {
    const min = Number(customMinZoom);
    const max = Number(customMaxZoom);
    return (
      Number.isFinite(min) &&
      Number.isFinite(max) &&
      min >= MIN_ZOOM_LIMIT &&
      max <= MAX_ZOOM_LIMIT &&
      min <= max
    );
  }

  function titleInputValid() {
    return downloadTitle.trim().length > 0;
  }

  function clampZoomValue(value: number) {
    return clamp(Math.round(value), MIN_ZOOM_LIMIT, MAX_ZOOM_LIMIT);
  }

  function syncMinZoomWithCurrent() {
    const rounded = clampZoomValue(currentZoom);
    customMinZoom = rounded;
    if (Number(customMaxZoom) < rounded) {
      customMaxZoom = clampZoomValue(rounded);
    }
  }

  function updateCurrentZoom() {
    if (!map) return;
    currentZoom = Number(map.getZoom().toFixed(2));
  }

  // Opens a native file picker so the user can choose a .pmtiles file to
  // use as the offline vector OSM base map. Only one file is active at a
  // time; picking a new one immediately replaces the previous choice.
  async function loadMapFile() {
    if (isLoadingMap) return;
    isLoadingMap = true;
    try {
      const path = await SelectVectorMapFile();
      if (path) {
        mapFilePath = path;
        if (map) {
          // Force MapLibre to drop cached tiles and refetch from
          // /vtiles/... under the newly selected archive.
          map.setStyle(getStyle(currentMode, apiKey, mapFilePath));
        }
        showCompletion("Offline map loaded");
      }
    } catch (e) {
      console.error("Failed to select vector map file", e);
      alert("Failed to load map file: " + e);
    } finally {
      isLoadingMap = false;
    }
  }

  function handlePointerDown(event: PointerEvent) {
    if (!selectMode || !map || !mapContainer) return;
    event.preventDefault();
    event.stopPropagation();
    const rect = mapContainer.getBoundingClientRect();
    const startX = clamp(event.clientX - rect.left, 0, rect.width);
    const startY = clamp(event.clientY - rect.top, 0, rect.height);
    dragStartPx = { x: startX, y: startY };
    dragStartLngLat = map.unproject([startX, startY]);
    selectionPixels = { left: startX, top: startY, width: 0, height: 0 };
    selectionBounds = null;
    isDrawingSelection = true;
    activePointerId = event.pointerId;
    mapContainer.setPointerCapture(activePointerId);
  }

  function handlePointerMove(event: PointerEvent) {
    if (
      !selectMode ||
      !isDrawingSelection ||
      !dragStartPx ||
      !map ||
      !mapContainer
    )
      return;
    event.preventDefault();
    event.stopPropagation();
    const rect = mapContainer.getBoundingClientRect();
    const currentX = clamp(event.clientX - rect.left, 0, rect.width);
    const currentY = clamp(event.clientY - rect.top, 0, rect.height);
    const left = Math.min(dragStartPx.x, currentX);
    const top = Math.min(dragStartPx.y, currentY);
    selectionPixels = {
      left,
      top,
      width: Math.abs(currentX - dragStartPx.x),
      height: Math.abs(currentY - dragStartPx.y),
    };
    const currentLngLat = map.unproject([currentX, currentY]);
    if (dragStartLngLat) {
      selectionBounds = normalizeBounds(dragStartLngLat, currentLngLat);
    }
  }

  function handlePointerUp(event: PointerEvent) {
    if (!selectMode || !isDrawingSelection) return;
    event.preventDefault();
    event.stopPropagation();
    if (
      selectionPixels &&
      (selectionPixels.width < 5 || selectionPixels.height < 5)
    ) {
      cancelSelection();
      return;
    }
    finalizeSelection();
  }

  async function performDownload(
    title: string,
    minZ: number,
    maxZ: number,
    bounds: Bounds,
  ) {
    if (isDownloading) return;
    isDownloading = true;
    downloadStatus = "Preparing...";
    try {
      // Offline downloads only ever apply to satellite imagery now —
      // the vector base map is a static, pre-supplied local file.
      const newBookmark = await DownloadRegion(
        "hybrid",
        title,
        minZ,
        maxZ,
        bounds.north,
        bounds.south,
        bounds.east,
        bounds.west,
      );
      const currentBookmarks = bookmarks;
      const safeBookmarks = Array.isArray(currentBookmarks)
        ? currentBookmarks
        : [];
      bookmarks = [newBookmark, ...safeBookmarks];
      isDownloading = false;
      downloadStatus = "Download queued";
      downloadLocked = true;
      downloadTitle = "";
      cancelSelection();
      const queuedMessage = `${newBookmark.title || "Download"} started`;
      showCompletion(queuedMessage);
    } catch (err) {
      console.error("Download failed", err);
      const message =
        err instanceof Error
          ? err.message
          : typeof err === "string"
            ? err
            : "Error occurred";
      downloadStatus = message;
      isDownloading = false;
      downloadLocked = false;
    }
  }

  function handleDownloadStatusEvent(eventData: DownloadEvent) {
    if (!eventData) return;
    const title = eventData.title?.trim() || "Download";
    if (eventData.status === "complete") {
      const message = eventData.message || `${title} ready`;
      downloadStatus = message;
      downloadLocked = false;
      showCompletion(message);
      return;
    }
    if (eventData.status === "error") {
      const message = eventData.message || `${title} failed`;
      downloadStatus = message;
      downloadLocked = false;
      showCompletion(message);
    }
  }

  async function clearAllDownloads() {
    if (isClearing) return;
    const confirmed = window.confirm(
      "Delete all saved maps and bookmarks? This cannot be undone.",
    );
    if (!confirmed) return;
    isClearing = true;
    downloadStatus = "Clearing downloads...";
    try {
      await ClearDownloads();
      await fetchBookmarks();
      downloadStatus = "All downloads removed";
      downloadLocked = false;
      showCompletion("All downloads removed");
    } catch (err) {
      console.error("Clear downloads failed", err);
      const message =
        err instanceof Error
          ? err.message
          : typeof err === "string"
            ? err
            : "Failed to delete downloads";
      downloadStatus = message;
      downloadLocked = false;
      showCompletion(message);
    } finally {
      isClearing = false;
    }
  }

  async function handleCustomDownload() {
    if (!selectionBounds) {
      alert("Please select an area first.");
      return;
    }
    if (!zoomInputsValid()) {
      alert(
        "Please provide valid zoom levels between 1 and 22 (min cannot exceed max).",
      );
      return;
    }
    if (!titleInputValid()) {
      alert("Please provide a title for this download.");
      return;
    }
    const minZ = clampZoomValue(Number(customMinZoom));
    const maxZ = clampZoomValue(Number(customMaxZoom));
    const title = downloadTitle.trim();
    await performDownload(title, minZ, maxZ, selectionBounds);
  }

  // --- Marker Functions ---
  function createCustomMarkerElement(name: string): HTMLElement {
    const el = document.createElement("div");
    const label = document.createElement("div");
    label.className = "custom-marker-label";
    label.textContent = name;
    const pin = document.createElement("div");
    pin.className = "custom-marker-pin";
    el.className = "custom-marker";
    el.appendChild(pin);
    el.appendChild(label);
    return el;
  }

  function toggleMarkerPanel() {
    showMarkerPanel = !showMarkerPanel;
    if (showMarkerPanel) {
      showAddMarkerForm = false;
      pinPointMode = false;
    }
  }

  function openAddMarkerForm() {
    showAddMarkerForm = true;
    showMarkerPanel = false;
    editingMarkerId = null;
    newMarkerName = "";
    newMarkerLat = "";
    newMarkerLng = "";
    newMarkerDirections = [];
    newDirectionAngle = "";
    newDirectionColor = DEFAULT_DIRECTION_COLOR;
    pinPointMode = false;
  }

  function cancelAddMarkerForm() {
    showAddMarkerForm = false;
    showMarkerPanel = true;
    editingMarkerId = null;
    pinPointMode = false;
    newMarkerName = "";
    newMarkerLat = "";
    newMarkerLng = "";
    newMarkerDirections = [];
    newDirectionAngle = "";
    newDirectionColor = DEFAULT_DIRECTION_COLOR;
    removePinPointTempMarker();
    if (map) map.getCanvas().style.cursor = "";
  }

  function addDirectionToForm() {
    const angle = parseFloat(newDirectionAngle);
    if (isNaN(angle)) {
      alert("Please enter a valid angle (0–360°).");
      return;
    }
    directionIdCounter++;
    newMarkerDirections = [
      ...newMarkerDirections,
      {
        id: directionIdCounter,
        angle: ((angle % 360) + 360) % 360,
        color: newDirectionColor,
      },
    ];
    newDirectionAngle = "";
    newDirectionColor = DEFAULT_DIRECTION_COLOR;
  }

  function removeDirectionFromForm(id: number) {
    newMarkerDirections = newMarkerDirections.filter((d) => d.id !== id);
  }

  function createPinPointTempElement(): HTMLElement {
    const el = document.createElement("div");
    el.className = "pinpoint-temp-marker";
    // Single centred dot — the 20×20 element lets MapLibre's
    // anchor:"center" translate(-50%,-50%) land exactly on the coordinate.
    const dot = document.createElement("div");
    dot.className = "pinpoint-temp-dot";
    el.appendChild(dot);
    return el;
  }

  function removePinPointTempMarker() {
    if (pinPointTempMarker) {
      pinPointTempMarker.remove();
      pinPointTempMarker = null;
    }
  }

  function togglePinPointMode() {
    pinPointMode = !pinPointMode;
    if (pinPointMode) {
      if (map) map.getCanvas().style.cursor = "crosshair";
    } else {
      removePinPointTempMarker();
      if (map) map.getCanvas().style.cursor = "";
    }
  }

  // --- Direction Marker Lines ---
  function addDirectionLine(
    markerId: number,
    directionId: number,
    lat: number,
    lng: number,
    angle: number,
    color: string,
  ) {
    if (!map) return;
    const sourceId = `dir-line-${markerId}-${directionId}`;
    const layerId = `dir-line-layer-${markerId}-${directionId}`;
    const endpoint = destinationPoint(lat, lng, angle, 10);
    const geojson: GeoJSON.FeatureCollection = {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          geometry: {
            type: "LineString",
            coordinates: [[lng, lat], endpoint],
          },
          properties: {},
        },
      ],
    };
    if (!map.getSource(sourceId)) {
      map.addSource(sourceId, { type: "geojson", data: geojson });
      map.addLayer({
        id: layerId,
        type: "line",
        source: sourceId,
        paint: { "line-color": color, "line-width": 2.5 },
      });
    } else {
      (map.getSource(sourceId) as maplibregl.GeoJSONSource).setData(geojson);
    }
  }

  function updateDirectionLines() {
    if (!map) return;
    for (const m of userMarkers) {
      for (const dir of m.directions) {
        addDirectionLine(m.id, dir.id, m.lat, m.lng, dir.angle, dir.color);
      }
    }
  }

  function removeMarkerDirectionLines(
    markerId: number,
    directions: MarkerDirection[],
  ) {
    if (!map) return;
    for (const dir of directions) {
      const layerId = `dir-line-layer-${markerId}-${dir.id}`;
      const sourceId = `dir-line-${markerId}-${dir.id}`;
      if (map.getLayer(layerId)) map.removeLayer(layerId);
      if (map.getSource(sourceId)) map.removeSource(sourceId);
    }
  }
  // --- End Direction Marker Lines ---

  function confirmAddMarker() {
    const lat = parseFloat(newMarkerLat);
    const lng = parseFloat(newMarkerLng);
    if (!newMarkerName.trim()) {
      alert("Please enter a name for the marker.");
      return;
    }
    if (isNaN(lat) || lat < -90 || lat > 90) {
      alert("Please enter a valid latitude (-90 to 90).");
      return;
    }
    if (isNaN(lng) || lng < -180 || lng > 180) {
      alert("Please enter a valid longitude (-180 to 180).");
      return;
    }
    if (!map) return;
    markerIdCounter++;
    const id = markerIdCounter;
    const name = newMarkerName.trim();
    const el = createCustomMarkerElement(name);
    const mapMarker = new maplibregl.Marker({ element: el, anchor: "center" })
      .setLngLat([lng, lat])
      .addTo(map);
    const directions = newMarkerDirections;
    userMarkers = [
      ...userMarkers,
      { id, name, lat, lng, directions, mapMarker },
    ];
    for (const dir of directions) {
      addDirectionLine(id, dir.id, lat, lng, dir.angle, dir.color);
    }
    removePinPointTempMarker();
    cancelAddMarkerForm();
    saveMarkersToStorage();
  }

  function removeUserMarker(id: number) {
    const idx = userMarkers.findIndex((m) => m.id === id);
    if (idx !== -1) {
      userMarkers[idx].mapMarker.remove();
      removeMarkerDirectionLines(id, userMarkers[idx].directions);
      userMarkers = userMarkers.filter((m) => m.id !== id);
      saveMarkersToStorage();
    }
  }

  function flyToMarker(m: UserMarker) {
    if (map) {
      map.flyTo({ center: [m.lng, m.lat], zoom: 14, essential: true });
    }
  }

  function openEditMarkerForm(m: UserMarker) {
    showAddMarkerForm = true;
    editingMarkerId = m.id;
    newMarkerName = m.name;
    newMarkerLat = String(m.lat);
    newMarkerLng = String(m.lng);
    newMarkerDirections = m.directions.map((d) => ({ ...d }));
    newDirectionAngle = "";
    newDirectionColor = DEFAULT_DIRECTION_COLOR;
    pinPointMode = false;
    // Hide the list while editing so the form has room
    showMarkerPanel = false;
  }

  function confirmEditMarker() {
    if (editingMarkerId === null || !map) return;
    const lat = parseFloat(newMarkerLat);
    const lng = parseFloat(newMarkerLng);
    if (!newMarkerName.trim()) {
      alert("Please enter a name for the marker.");
      return;
    }
    if (isNaN(lat) || lat < -90 || lat > 90) {
      alert("Please enter a valid latitude (-90 to 90).");
      return;
    }
    if (isNaN(lng) || lng < -180 || lng > 180) {
      alert("Please enter a valid longitude (-180 to 180).");
      return;
    }
    const idx = userMarkers.findIndex((m) => m.id === editingMarkerId);
    if (idx === -1) return;
    const existing = userMarkers[idx];
    // Remove the old map marker and direction lines
    existing.mapMarker.remove();
    removeMarkerDirectionLines(existing.id, existing.directions);
    // Recreate with updated values
    const name = newMarkerName.trim();
    const el = createCustomMarkerElement(name);
    const mapMarker = new maplibregl.Marker({ element: el, anchor: "center" })
      .setLngLat([lng, lat])
      .addTo(map);
    const directions = newMarkerDirections;
    const updated: UserMarker = {
      ...existing,
      name,
      lat,
      lng,
      directions,
      mapMarker,
    };
    for (const dir of directions) {
      addDirectionLine(existing.id, dir.id, lat, lng, dir.angle, dir.color);
    }
    userMarkers = userMarkers.map((m) => (m.id === existing.id ? updated : m));
    removePinPointTempMarker();
    saveMarkersToStorage();
    cancelAddMarkerForm();
  }
  // --- End Marker Functions ---

  // Re-apply the map style when the API key loads so online tiles work
  // without blocking the initial map render on the config IPC roundtrip.
  let prevApiKey = "";
  $effect(() => {
    const key = apiKey;
    if (!map || !key || key === prevApiKey) return;
    prevApiKey = key;
    map.setStyle(getStyle(currentMode, apiKey, mapFilePath));
  });

  onMount(async () => {
    // Fire config load without awaiting — the $effect above will update
    // the map style once the API key arrives.
    configStore.load();

    // Load the currently selected vector map path (if any) so the UI
    // can show the right filename / placeholder state.
    try {
      mapFilePath = (await GetVectorMapPath()) ?? "";
    } catch (e) {
      console.error("Failed to load vector map path", e);
    }

    const lat = locationStore.data.latitude ?? -6.75;
    const lng = locationStore.data.longitude ?? 110.36;
    map = new maplibregl.Map({
      container: mapContainer,
      style: getStyle(currentMode, apiKey, mapFilePath),
      center: [lng, lat],
      zoom: 4,
      attributionControl: false,
    });
    map.addControl(new maplibregl.AttributionControl(), "bottom-left");
    // map.addControl(new maplibregl.ScaleControl(), "bottom-left");
    map.addControl(new maplibregl.NavigationControl(), "bottom-left");

    updateCurrentZoom();
    syncMinZoomWithCurrent();
    map.on("zoom", updateCurrentZoom);
    map.on("zoomend", updateCurrentZoom);
    map.on("styledata", updateDFLine);
    map.on("styledata", updateDirectionLines);
    mapClickHandler = (e) => {
      if (pinPointMode) {
        newMarkerLat = e.lngLat.lat.toFixed(6);
        newMarkerLng = e.lngLat.lng.toFixed(6);
        if (!pinPointTempMarker) {
          pinPointTempMarker = new maplibregl.Marker({
            element: createPinPointTempElement(),
            anchor: "center",
          })
            .setLngLat(e.lngLat)
            .addTo(map);
        } else {
          pinPointTempMarker.setLngLat(e.lngLat);
        }
        pinPointMode = false;
        map.getCanvas().style.cursor = "";
      }
    };
    map.on("click", mapClickHandler);
    // Restore persisted markers after the first style load
    map.once("load", restoreMarkersFromStorage);
    unsubscribeDownloadEvents = EventsOn("download-status", (...payload) =>
      handleDownloadStatusEvent((payload?.[0] as DownloadEvent) || null),
    );
    fetchBookmarks();
    findMyLocation();

    // Keep the canvas in sync with the container's real size — otherwise
    // clicks/markers drift from the cursor whenever the layout settles
    // or panels toggle after the map was first created.
    mapResizeObserver = new ResizeObserver(() => map?.resize());
    mapResizeObserver.observe(mapContainer);
  });

  onDestroy(() => {
    if (noticeTimer) {
      clearTimeout(noticeTimer);
    }
    if (_dfLineThrottleTimer) {
      clearTimeout(_dfLineThrottleTimer);
      _dfLineThrottleTimer = null;
    }
    if (mapResizeObserver) {
      mapResizeObserver.disconnect();
      mapResizeObserver = null;
    }
    if (map) {
      map.off("zoom", updateCurrentZoom);
      map.off("zoomend", updateCurrentZoom);
      map.off("styledata", updateDFLine);
      map.off("styledata", updateDirectionLines);
      if (mapClickHandler) map.off("click", mapClickHandler);
      removePinPointTempMarker();
      map.remove();
    }
    if (unsubscribeDownloadEvents) {
      unsubscribeDownloadEvents();
      unsubscribeDownloadEvents = null;
    }
  });
</script>

<div class="map-layout">
  <div class="controls">
    <div class="toolbar">
      <div class="toolbar-row">
        <div class="mode-switch" role="group" aria-label="Map mode">
          <button
            class="mode-switch-btn"
            class:active={currentMode === "hybrid"}
            onclick={() => switchStyle("hybrid")}
            aria-label="Satellite map"
            title="Satellite (online, falls back to offline automatically)"
          >
            Satellite
          </button>
          <button
            class="mode-switch-btn"
            class:active={currentMode === "normal"}
            onclick={() => switchStyle("normal")}
            aria-label="Normal map"
            title="Normal (offline vector map)"
          >
            Normal
          </button>
        </div>

        {#if currentMode === "normal"}
          <button
            class="toolbar-btn load-maps-btn"
            class:active-state={!!mapFilePath}
            disabled={isLoadingMap}
            onclick={loadMapFile}
            aria-label="Load offline map file"
            title={mapFilePath || "Select PMTiles map file"}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M12 3v12" />
              <path d="M7 10l5 5 5-5" />
              <path d="M4 19h16" />
            </svg>
            {isLoadingMap ? "Loading..." : "Load Maps"}
          </button>
        {:else}
          <button
            class="toolbar-btn download-btn-toggle"
            class:active-state={showDownloadPanel}
            disabled={isDownloading}
            aria-label="Open maps download"
            title="Maps Download"
            onclick={() => {
              console.log("Maps Download clicked", {
                isDownloading,
                showDownloadPanel,
              });
              toggleDownloadPanel("download");
            }}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M12 3v12" />
              <path d="M7 10l5 5 5-5" />
              <path d="M4 19h16" />
            </svg>
            Maps
          </button>
        {/if}
      </div>

      <div class="toolbar-row">
        <button
          aria-label="Show Markers"
          title="Show Markers"
          class="toolbar-btn"
          class:active-state={showMarkerBottomPanel}
          onclick={toggleMarkerBottomPanel}
          ><svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z" />
            <circle cx="12" cy="10" r="3" />
          </svg></button
        >
        <button
          class="toolbar-btn"
          onclick={findMyLocation}
          aria-label="Find my location"
          title="Find Me"
        >
          <div class="my-loc-btn">
            <div></div>
          </div>
        </button>
        <button
          class="toolbar-btn reset-btn"
          onclick={resetMapView}
          aria-label="Reset map to north-up flat view"
          title="Reset view"
        >
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M3 12a9 9 0 1 0 3-6.7" />
            <path d="M3 3v5h5" />
          </svg>
        </button>

        <div class="toolbar-indicator hide-button" title="Zoom Value">
          Zoom {currentZoom.toFixed(1)}
        </div>
      </div>
    </div>
    {#if completionNotice}
      <div class="notice">{completionNotice}</div>
    {/if}
    {#if showDownloadPanel}
      <div class="download-menu">
        <!-- Tab bar -->
        <div class="dl-tabs">
          <button
            class="dl-tab"
            class:active={downloadTab === "bookmarks"}
            onclick={() => (downloadTab = "bookmarks")}>Saved Maps</button
          >
          <button
            class="dl-tab"
            class:active={downloadTab === "download"}
            onclick={() => {
              downloadTab = "download";
              syncMinZoomWithCurrent();
              switchStyle("hybrid");
            }}>Download</button
          >
        </div>

        {#if downloadTab === "download"}
          <div class="select-row">
            <button
              class="select-btn"
              class:active={selectMode}
              onclick={selectMode || selectionBounds
                ? cancelSelection
                : beginSelection}
            >
              {selectMode || selectionBounds ? "Cancel Select" : "Select Area"}
            </button>
            <div class="status-indicator">{downloadStatus}</div>
          </div>
          <label class="title-input">
            Title
            <input
              type="text"
              placeholder="e.g. Downtown"
              maxlength="80"
              bind:value={downloadTitle}
            />
          </label>
          <div class="zoom-inputs">
            <label>
              Min Zoom
              <input
                type="number"
                min={MIN_ZOOM_LIMIT}
                max={MAX_ZOOM_LIMIT}
                bind:value={customMinZoom}
              />
            </label>
            <label>
              Max Zoom
              <input
                type="number"
                min={MIN_ZOOM_LIMIT}
                max={MAX_ZOOM_LIMIT}
                bind:value={customMaxZoom}
              />
            </label>
          </div>
          <button
            class="download-btn secondary"
            disabled={isDownloading ||
              !selectionBounds ||
              !zoomInputsValid() ||
              !titleInputValid() ||
              downloadLocked}
            onclick={handleCustomDownload}
          >
            {isDownloading ? "Downloading..." : "Download"}
          </button>
          <button
            class="download-btn danger"
            disabled={isClearing || downloadLocked}
            onclick={clearAllDownloads}
          >
            {isClearing ? "Clearing..." : "Delete All Downloads"}
          </button>
        {:else}
          <div class="bookmark-list-inner">
            {#if bookmarks.length === 0}
              <div class="bookmark-empty">No saved maps yet.</div>
            {:else}
              {#each bookmarks as b}
                <button class="bookmark-btn" onclick={() => goToBookmark(b)}>
                  <span class="bookmark-title"
                    >{b.title || "Untitled download"}</span
                  >
                  <span class="bookmark-meta">
                    {b.style} | Zoom: {b.min_zoom}–{b.max_zoom}
                    | [{b.center_lat.toFixed(4)}, {b.center_lng.toFixed(4)}]
                  </span>
                </button>
              {/each}
            {/if}
          </div>
        {/if}
      </div>
    {/if}
  </div>
  <div
    class="map-container"
    class:selecting={selectMode}
    class:pinpointing={pinPointMode}
    bind:this={mapContainer}
    role="application"
    aria-label="Map Region Selection"
    onpointerdown={handlePointerDown}
    onpointermove={handlePointerMove}
    onpointerup={handlePointerUp}
    onpointercancel={handlePointerUp}
  >
    {#if selectionPixels}
      <div
        class="selection-overlay"
        style={`left: ${selectionPixels.left}px; top: ${selectionPixels.top}px; width: ${selectionPixels.width}px; height: ${selectionPixels.height}px;`}
      ></div>
    {/if}
    {#if currentMode === "normal" && !mapFilePath}
      <div class="no-map-placeholder">
        <p>No offline map loaded.</p>
        <button onclick={loadMapFile} disabled={isLoadingMap}>
          {isLoadingMap ? "Loading..." : "Select PMTiles File"}
        </button>
      </div>
    {/if}
  </div>

  <!-- Marker Bottom Panel -->
  {#if showMarkerBottomPanel}
    <div class="marker-bottom-panel" class:pinpointing={pinPointMode}>
      <div class="marker-panel-header">
        <span class="marker-panel-title">Markers ({userMarkers.length})</span>
        <div class="marker-panel-actions">
          {#if !showAddMarkerForm}
            <button class="marker-action-btn add" onclick={openAddMarkerForm}>
              + Add Marker
            </button>
          {/if}
          <button
            class="marker-action-btn toggle"
            class:open={showMarkerPanel}
            onclick={toggleMarkerPanel}
            aria-label="Toggle marker list"
          >
            {showMarkerPanel ? "▼" : "▲"} List
          </button>
        </div>
      </div>

      {#if showAddMarkerForm}
        <div class="add-marker-form">
          <div class="marker-type-chip">
            {editingMarkerId !== null ? "✏️ Edit Marker" : "📍 New Marker"}
          </div>
          <input
            class="marker-input"
            type="text"
            placeholder="Marker name"
            maxlength="80"
            bind:value={newMarkerName}
          />
          <div class="coord-row">
            <input
              class="marker-input coord"
              type="number"
              placeholder="Latitude"
              step="0.000001"
              bind:value={newMarkerLat}
            />
            <input
              class="marker-input coord"
              type="number"
              placeholder="Longitude"
              step="0.000001"
              bind:value={newMarkerLng}
            />
            <button
              class="pinpoint-btn"
              class:active={pinPointMode}
              onclick={togglePinPointMode}
              title="Click on the map to pick coordinates"
            >
              {#if pinPointMode}
                <span class="pinpoint-icon">✕</span> Cancel
              {:else}
                <span class="pinpoint-icon">📍</span> Pin
              {/if}
            </button>
          </div>
          {#if pinPointMode}
            <div class="pinpoint-hint">
              Click anywhere on the map to set coordinates
            </div>
          {/if}
          <div class="directions-section">
            <span class="directions-label">Direction lines (optional)</span>
            {#if newMarkerDirections.length > 0}
              <div class="direction-chip-list">
                {#each newMarkerDirections as d}
                  <div class="direction-chip">
                    <span
                      class="direction-color-dot"
                      style="background:{d.color}"
                    ></span>
                    <span>{d.angle.toFixed(0)}°</span>
                    <button
                      class="direction-chip-remove"
                      onclick={() => removeDirectionFromForm(d.id)}
                      aria-label="Remove direction">✕</button
                    >
                  </div>
                {/each}
              </div>
            {/if}
            <div class="direction-add-row">
              <input
                class="marker-input coord"
                type="number"
                placeholder="Angle 0–360°"
                min="0"
                max="360"
                step="1"
                bind:value={newDirectionAngle}
              />
              <input
                class="direction-color-input"
                type="color"
                bind:value={newDirectionColor}
                title="Line color"
              />
              <button
                class="direction-add-btn"
                onclick={addDirectionToForm}
                type="button">+ Add</button
              >
            </div>
          </div>
          <div class="form-actions">
            {#if editingMarkerId !== null}
              <button class="marker-action-btn add" onclick={confirmEditMarker}
                >Save</button
              >
            {:else}
              <button class="marker-action-btn add" onclick={confirmAddMarker}
                >Add</button
              >
            {/if}
            <button
              class="marker-action-btn cancel"
              onclick={cancelAddMarkerForm}>Cancel</button
            >
          </div>
        </div>
      {/if}

      {#if showMarkerPanel}
        <div class="marker-list-scroll">
          {#if userMarkers.length === 0}
            <div class="marker-empty">No markers added yet.</div>
          {:else}
            {#each userMarkers as m}
              <div class="marker-list-item">
                <button class="marker-name-btn" onclick={() => flyToMarker(m)}>
                  <span class="marker-list-icon">📍</span>
                  <span class="marker-list-name">{m.name}</span>
                  <span class="marker-list-coords">
                    {m.lat.toFixed(4)}, {m.lng.toFixed(4)}
                    {#if m.directions.length > 0}
                      &nbsp;· 🧭{m.directions.length}
                    {/if}
                  </span>
                </button>
                <button
                  class="marker-edit-btn"
                  onclick={() => openEditMarkerForm(m)}
                  aria-label="Edit marker {m.name}">✎</button
                >
                <button
                  class="marker-remove-btn"
                  onclick={() => removeUserMarker(m.id)}
                  aria-label="Remove marker {m.name}">✕</button
                >
              </div>
            {/each}
          {/if}
        </div>
      {/if}
    </div>
  {/if}
  <!-- End Marker Bottom Panel -->
</div>

<style>
  :global(.my-location-marker) {
    width: 24px;
    height: 24px;
  }
  :global(.my-location-pulse) {
    position: absolute;
    inset: -8px;
    border-radius: 50%;
    background: rgba(37, 99, 235, 0.2);
    animation: location-pulse 2s ease-out infinite;
  }
  :global(.my-location-dot) {
    position: absolute;
    inset: 0;
    border-radius: 50%;
    background: #2563eb;
    border: 3px solid #ffffff;
    box-shadow: 0 1px 6px rgba(0, 0, 0, 0.35);
  }
  @keyframes location-pulse {
    0% {
      transform: scale(0.6);
      opacity: 1;
    }
    80% {
      transform: scale(1.8);
      opacity: 0;
    }
    100% {
      transform: scale(1.8);
      opacity: 0;
    }
  }

  /* Download panel tabs */
  .dl-tabs {
    display: flex;
    border-bottom: 1px solid #e2e8f0;
    margin: 0 -10px 8px;
    padding: 0 10px;
  }
  .bookmark-list-inner {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 220px;
    overflow-y: auto;
    padding-right: 2px;
  }
  .bookmark-empty {
    text-align: center;
    font-size: 12px;
    color: #94a3b8;
    padding: 12px 0;
  }
  .bookmark-btn {
    background: #f8fafc;
    border: 1px solid #e2e8f0;
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 13px;
    cursor: pointer;
    text-align: left;
    display: flex;
    flex-direction: column;
    gap: 2px;
    color: #0f172a;
    transition:
      background 0.2s ease,
      border-color 0.2s ease;
  }
  .bookmark-title {
    font-weight: 600;
    font-size: 14px;
    color: #0f172a;
  }
  .bookmark-meta {
    font-size: 12px;
    color: #475569;
  }
  .bookmark-btn:hover {
    background: #e0f2fe;
    border-color: #7dd3fc;
  }
  .map-layout {
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
    -webkit-app-region: no-drag;
  }

  .map-container {
    flex-grow: 1;
    width: 100%;
    height: 100%;
    -webkit-app-region: no-drag;
  }

  .map-container.selecting {
    touch-action: none;
    cursor: default;
  }

  /* Floating Controls */
  .controls {
    position: absolute;
    top: 10px;
    left: 10px;
    z-index: 12000;
    display: flex;
    flex-direction: column;
    gap: 6px;
    align-items: stretch;
    padding: 4px;
    width: 320px;
    max-width: 90vw;
    background: rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(6px);
    border-radius: 12px;
    box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
    border: 1px solid rgba(148, 163, 184, 0.35);
    -webkit-app-region: no-drag;
    pointer-events: auto;
  }

  .toolbar {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
    width: 100%;
    background: rgba(248, 250, 252, 0.5);
    border-radius: 12px;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.7),
      0 6px 16px rgba(15, 23, 42, 0.08);
    border: 1px solid rgba(148, 163, 184, 0.35);
    padding: 6px;
    pointer-events: auto;
    -webkit-app-region: no-drag;
  }

  .toolbar-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }

  .toolbar-row > * {
    width: auto;
    min-width: 0;
  }

  .toolbar-btn,
  .toolbar-indicator {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 34px;
    padding: 0 10px;
    border-radius: 10px;
    background: #ffffff;
    border: 1px solid rgba(148, 163, 184, 0.45);
    color: #000000;
    font-size: 13px;
    font-weight: 600;
    box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
    transition:
      background 0.15s ease,
      color 0.15s ease,
      border-color 0.15s ease,
      box-shadow 0.15s ease;
  }

  .toolbar-btn {
    cursor: pointer;
    touch-action: manipulation;
  }

  .toolbar-btn:hover {
    background: #f1f5f9;
    border-color: rgba(59, 130, 246, 0.4);
    box-shadow: 0 4px 10px rgba(37, 99, 235, 0.12);
  }

  .toolbar-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    background: #eef2f7;
    border-color: rgba(148, 163, 184, 0.45);
    box-shadow: none;
  }

  .toolbar-btn.active-state {
    background: #e8f0ff;
    border-color: #93c5fd;
    color: #1d4ed8;
    box-shadow: 0 4px 12px rgba(59, 130, 246, 0.16);
  }

  /* Normal / Satellite segmented mode switch */
  .mode-switch {
    display: inline-flex;
    border-radius: 10px;
    overflow: hidden;
    border: 1px solid rgba(148, 163, 184, 0.45);
    box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
  }

  .mode-switch-btn {
    height: 34px;
    padding: 0 14px;
    border: none;
    background: #ffffff;
    color: #0f172a;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    touch-action: manipulation;
    transition:
      background 0.15s ease,
      color 0.15s ease;
  }

  .mode-switch-btn + .mode-switch-btn {
    border-left: 1px solid rgba(148, 163, 184, 0.45);
  }

  .mode-switch-btn:hover:not(.active) {
    background: #f1f5f9;
  }

  .mode-switch-btn.active {
    background: #6366f1;
    color: #ffffff;
  }
  .toolbar-btn.reset-btn {
    color: #334155;
  }

  .toolbar-btn.reset-btn:hover {
    border-color: rgba(100, 116, 139, 0.5);
    box-shadow: 0 4px 10px rgba(51, 65, 85, 0.14);
  }

  .toolbar-btn.download-btn-toggle.active-state {
    background: #2563eb;
    border-color: #1d4ed8;
    color: #ffffff;
    box-shadow: 0 6px 14px rgba(37, 99, 235, 0.28);
  }

  .toolbar-indicator {
    font-size: 12px;
    font-weight: 600;
    color: #475569;
    cursor: default;
    letter-spacing: 0.01em;
  }

  .download-btn {
    background: #007bff;
    color: white;
    border: none;
    padding: 10px 15px;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 600;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  }

  .download-btn:disabled {
    background: #6c757d;
    cursor: not-allowed;
  }

  .download-btn.secondary {
    background: #22c55e;
  }

  .download-btn.danger {
    background: #dc2626;
  }

  .download-btn.danger:disabled {
    background: #fca5a5;
  }

  .download-menu {
    position: relative;
    z-index: 2;
    background: rgba(255, 255, 255, 0.5);
    border-radius: 10px;
    padding: 12px;
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.18);
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: 100%;
    box-sizing: border-box;
    touch-action: manipulation;
    pointer-events: auto;
    -webkit-app-region: no-drag;
  }
  .my-loc-btn {
    width: 18px;
    height: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background-color: white;
    border: 1px solid black;
  }
  .my-loc-btn > div {
    background-color: #2563eb;
    width: 14px;
    height: 14px;
    border-radius: 50%;
  }

  .notice {
    background: #28a745;
    color: white;
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 12px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  }

  .select-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }

  .select-btn {
    border: 1px solid #007bff;
    background: white;
    color: #007bff;
    padding: 6px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 600;
    transition: background 0.15s ease;
  }

  .select-btn.active,
  .select-btn:hover {
    background: #007bff;
    color: white;
  }
  .zoom-inputs input {
    margin-top: 4px;
    padding: 6px;
    border: 1px solid #ccc;
    border-radius: 4px;
    font-size: 13px;
    width: 100%;
    box-sizing: border-box;
  }

  .title-input {
    display: flex;
    flex-direction: column;
    font-size: 12px;
    color: #555;
  }

  .title-input input {
    margin-top: 4px;
    padding: 6px;
    border: 1px solid #ccc;
    border-radius: 4px;
    font-size: 13px;
    width: 100%;
    box-sizing: border-box;
  }

  .status-indicator {
    font-size: 12px;
    color: #0f172a;
    background: rgba(15, 23, 42, 0.08);
    padding: 2px 4px;
    border-radius: 999px;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    min-width: 80px;
  }

  .map-container {
    position: relative;
  }

  .map-container.pinpointing {
    cursor: crosshair !important;
  }

  .status-indicator {
    font-size: 12px;
    color: #0f172a;
    background: rgba(15, 23, 42, 0.08);
    padding: 2px 4px;
    border-radius: 999px;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    min-width: 80px;
  }
  .selection-overlay {
    position: absolute;
    border: 2px dashed #007bff;
    background: rgba(0, 123, 255, 0.15);
    pointer-events: none;
    z-index: 5;
  }

  .no-map-placeholder {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 6;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    background: rgba(255, 255, 255, 0.92);
    border: 1px solid rgba(148, 163, 184, 0.45);
    border-radius: 12px;
    padding: 20px 28px;
    box-shadow: 0 8px 24px rgba(15, 23, 42, 0.15);
    text-align: center;
    pointer-events: auto;
  }

  .no-map-placeholder p {
    margin: 0;
    font-size: 13px;
    color: #475569;
    font-weight: 600;
  }

  .no-map-placeholder button {
    background: #2563eb;
    color: white;
    border: none;
    padding: 8px 16px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
  }

  .no-map-placeholder button:disabled {
    background: #93a3b8;
    cursor: not-allowed;
  }


  /* ---- Marker Bottom Panel ---- */
  .marker-bottom-panel {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: 14000;
    background: rgba(255, 255, 255, 0.97);
    border-top: 1px solid rgba(0, 0, 0, 0.1);
    box-shadow: 0 -3px 10px rgba(0, 0, 0, 0.12);
    display: flex;
    flex-direction: column;
    gap: 0;
    max-height: 55vh;
    transition: max-height 0.2s ease;
    touch-action: manipulation;
  }

  .marker-bottom-panel.pinpointing {
    pointer-events: none;
  }

  .marker-bottom-panel.pinpointing .marker-panel-header,
  .marker-bottom-panel.pinpointing .add-marker-form {
    pointer-events: auto;
  }

  .marker-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 12px;
    gap: 8px;
    flex-shrink: 0;
  }

  .marker-panel-title {
    font-size: 13px;
    font-weight: 600;
    color: #0f172a;
  }

  .marker-panel-actions {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .marker-action-btn {
    border: none;
    border-radius: 6px;
    padding: 5px 12px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition:
      background 0.15s ease,
      color 0.15s ease;
  }

  .marker-action-btn.add {
    background: #22c55e;
    color: white;
  }

  .marker-action-btn.add:hover {
    background: #16a34a;
    box-shadow: 0 3px 10px rgba(22, 163, 74, 0.4);
  }

  .marker-action-btn.cancel {
    background: #ffffff;
    color: #475569;
    border: 1.5px solid #cbd5e1;
  }

  .marker-action-btn.cancel:hover {
    background: #f1f5f9;
    border-color: #94a3b8;
  }

  .marker-action-btn.toggle {
    background: #f1f5f9;
    color: #475569;
  }

  .marker-action-btn.toggle.open {
    background: #e0f2fe;
    color: #0369a1;
  }

  .add-marker-form {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 0 12px 10px 12px;
    border-top: 1px solid #f1f5f9;
    background: #f8fafc;
    flex-shrink: 0;
  }

  .marker-input {
    padding: 6px 8px;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    font-size: 13px;
    outline: none;
    width: 100%;
    box-sizing: border-box;
    margin-top: 8px;
  }

  .marker-input:focus {
    border-color: #3b82f6;
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
  }

  .coord-row {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .marker-input.coord {
    flex: 1;
    margin-top: 0;
    min-width: 0;
  }

  .pinpoint-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 6px 10px;
    border: 1px solid #3b82f6;
    border-radius: 6px;
    background: white;
    color: #3b82f6;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    transition:
      background 0.15s ease,
      color 0.15s ease;
    flex-shrink: 0;
  }

  .pinpoint-btn.active {
    background: #3b82f6;
    color: white;
  }

  .pinpoint-btn:hover:not(.active) {
    background: #eff6ff;
  }

  .pinpoint-icon {
    font-size: 14px;
  }

  .pinpoint-hint {
    font-size: 11px;
    color: #3b82f6;
    background: #eff6ff;
    border-radius: 4px;
    padding: 4px 8px;
    text-align: center;
    animation: hint-pulse 1.2s ease-in-out infinite;
  }

  @keyframes hint-pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.6;
    }
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    margin-top: 6px;
    padding-top: 10px;
    border-top: 1px solid #e2e8f0;
  }

  .form-actions .marker-action-btn {
    padding: 8px 20px;
    font-size: 13px;
    border-radius: 8px;
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
  }

  /* maximum height for the marker list */
  .marker-list-scroll {
    overflow-y: auto;
    max-height: 40vh;
    border-top: 1px solid #f1f5f9;
  }

  .marker-empty {
    padding: 12px;
    text-align: center;
    font-size: 13px;
    color: #94a3b8;
  }

  .marker-list-item {
    display: flex;
    align-items: center;
    border-bottom: 1px solid #f1f5f9;
  }

  .marker-list-item:last-child {
    border-bottom: none;
  }

  .marker-name-btn {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    min-width: 0;
    transition: background 0.15s ease;
  }

  .marker-name-btn:hover {
    background: #f0f9ff;
  }

  .marker-list-icon {
    font-size: 16px;
    flex-shrink: 0;
  }

  .marker-list-name {
    font-size: 13px;
    font-weight: 600;
    color: #0f172a;
    flex-shrink: 0;
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .marker-list-coords {
    font-size: 11px;
    color: #64748b;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .marker-remove-btn {
    flex-shrink: 0;
    background: none;
    border: none;
    color: #ef4444;
    font-size: 14px;
    padding: 8px 12px;
    cursor: pointer;
    transition: background 0.15s ease;
  }

  .marker-remove-btn:hover {
    background: #fef2f2;
  }

  .marker-edit-btn {
    flex-shrink: 0;
    background: none;
    border: none;
    color: #3b82f6;
    font-size: 15px;
    padding: 8px 10px;
    cursor: pointer;
    transition: background 0.15s ease;
  }

  .marker-edit-btn:hover {
    background: #eff6ff;
  }

  :global(.pinpoint-temp-marker) {
    width: 20px;
    height: 20px;
    pointer-events: none;
  }
  :global(.pinpoint-temp-dot) {
    position: absolute;
    inset: 3px;
    border-radius: 50%;
    background: #f97316;
    border: 2px solid #ffffff;
    box-shadow:
      0 0 0 2px #f97316,
      0 1px 6px rgba(0, 0, 0, 0.4);
  }
  /* ---- end temporary marker style ---- */

  /* Custom map marker pin — point (blue circle), label floats above.
       Fixed 18×18 box matching the pin so anchor:"center" lands exactly
       on the coordinate regardless of the label's size. */
  :global(.custom-marker) {
    width: 18px;
    height: 18px;
    cursor: pointer;
  }

  :global(.custom-marker-label) {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 4px;
    background: rgba(255, 255, 255, 0.92);
    color: #0f172a;
    font-size: 11px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 4px;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
    white-space: nowrap;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    pointer-events: none;
  }

  :global(.custom-marker-pin) {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #087827;
    border: 2px solid #ffffff;
    box-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
  }
  /* Marker type chip */
  .marker-type-chip {
    display: inline-block;
    font-size: 11px;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 999px;
    margin-top: 8px;
    align-self: flex-start;
    background: #e0f2fe;
    color: #0369a1;
  }

  /* Direction lines add-on */
  .directions-section {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 4px;
    border-top: 1px dashed #e2e8f0;
  }

  .directions-label {
    font-size: 12px;
    font-weight: 600;
    color: #475569;
  }

  .direction-chip-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .direction-chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    background: #f1f5f9;
    border-radius: 999px;
    padding: 3px 6px 3px 8px;
    font-size: 12px;
    color: #0f172a;
  }

  .direction-color-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex-shrink: 0;
    border: 1px solid rgba(0, 0, 0, 0.15);
  }

  .direction-chip-remove {
    background: none;
    border: none;
    color: #ef4444;
    cursor: pointer;
    font-size: 11px;
    padding: 0 2px;
  }

  .direction-add-row {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .direction-add-row .marker-input.coord {
    flex: 1;
  }

  .direction-color-input {
    width: 34px;
    height: 32px;
    padding: 0;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    cursor: pointer;
    flex-shrink: 0;
  }

  .direction-add-btn {
    flex-shrink: 0;
    border: 1px solid #3b82f6;
    background: white;
    color: #3b82f6;
    border-radius: 6px;
    padding: 6px 10px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    transition:
      background 0.15s ease,
      color 0.15s ease;
  }

  .direction-add-btn:hover {
    background: #3b82f6;
    color: white;
  }
</style>
