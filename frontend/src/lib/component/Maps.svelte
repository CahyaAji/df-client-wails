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
    } from "../../../wailsjs/go/main/App";
    import { EventsOn } from "../../../wailsjs/runtime/runtime";
    import { configStore } from "../store/configStore.svelte.js";
    import { compassStore } from "../store/compassStore.svelte.js";
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
            bookmarks = await ListBookmarks();
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
    let currentZoom = $state(14);
    let offlineMode = $state(false);
    let isClearing = $state(false);
    let downloadLocked = $state(false);

    let locationMarker: maplibregl.Marker | null = null;

    // --- User Markers ---
    type UserMarkerType = "point" | "direction";
    type UserMarker = {
        id: number;
        name: string;
        lat: number;
        lng: number;
        type: UserMarkerType;
        angle?: number; // only for direction markers
        mapMarker: maplibregl.Marker;
    };

    // Serialisable shape — excludes the live maplibregl.Marker instance
    type StoredMarker = Omit<UserMarker, "mapMarker">;
    const MARKERS_STORAGE_KEY = "df_client_user_markers";

    function saveMarkersToStorage() {
        const data: StoredMarker[] = userMarkers.map(
            ({ id, name, lat, lng, type, angle }) =>
                angle !== undefined
                    ? { id, name, lat, lng, type, angle }
                    : { id, name, lat, lng, type },
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
                const el = createCustomMarkerElement(d.name, d.type);
                const anchor = d.type === "direction" ? "center" : "bottom";
                const mapMarker = new maplibregl.Marker({ element: el, anchor })
                    .setLngLat([d.lng, d.lat])
                    .addTo(map);
                userMarkers = [...userMarkers, { ...d, mapMarker }];
                if (d.type === "direction" && d.angle !== undefined) {
                    addDirectionLine(d.id, d.lat, d.lng, d.angle);
                }
                if (d.id >= markerIdCounter) markerIdCounter = d.id + 1;
            }
        } catch {
            console.warn("Failed to restore markers from localStorage");
        }
    }

    let userMarkers: UserMarker[] = $state([]);
    let markerIdCounter = 0;
    let showMarkerPanel = $state(false);
    let showMarkerBottomPanel = $state(false);
    let showAddMarkerForm = $state(false);
    let markerType = $state<UserMarkerType | null>(null);
    let pinPointMode = $state(false);
    let newMarkerName = $state("");
    let newMarkerLat = $state("");
    let newMarkerLng = $state("");
    let newMarkerAngle = $state("");
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
            Math.sin(φ1) * Math.cos(d) +
                Math.cos(φ1) * Math.sin(d) * Math.cos(θ),
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

        const hasData =
            latitude != null &&
            longitude != null &&
            heading !== null;

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
                                  destinationPoint(
                                      latitude!,
                                      longitude!,
                                      heading!,
                                      10,
                                  ),
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

    $effect(() => {
        // Re-run whenever location or DF heading changes
        locationStore.data.latitude;
        locationStore.data.longitude;
        dfStore.data;
        updateDFLine();
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
    const MAX_ZOOM_LIMIT = 16;

    let customMinZoom = $state(MIN_ZOOM_LIMIT);
    let customMaxZoom = $state(14);
    let selectionPixels: SelectionPixels | null = $state(null);
    let selectionBounds: Bounds | null = $state(null);

    let isDrawingSelection = false;
    let dragStartPx: { x: number; y: number } | null = null;
    let dragStartLngLat: maplibregl.LngLat | null = null;
    let activePointerId: number | null = null;

    const clamp = (value: number, min: number, max: number) =>
        Math.max(min, Math.min(max, value));

    function normalizeBounds(
        a: maplibregl.LngLat,
        b: maplibregl.LngLat,
    ): Bounds {
        return {
            north: Math.max(a.lat, b.lat),
            south: Math.min(a.lat, b.lat),
            east: Math.max(a.lng, b.lng),
            west: Math.min(a.lng, b.lng),
        };
    }

    // const API_KEY = "fB2eDjoDg2nlel5Kw6ym";
    // const API_KEY = "aUOEn1bA48mz3xc3pL4N";
    const apiKey = $derived(configStore.mapKey);

    function getStyle(mode: "normal" | "hybrid") {
        const isHybrid = mode === "hybrid";

        const onlineUrl = isHybrid
            ? `https://api.maptiler.com/maps/hybrid/{z}/{x}/{y}.jpg?key=${apiKey}`
            : `https://api.maptiler.com/maps/openstreetmap/{z}/{x}/{y}.jpg?key=${apiKey}`;

        return {
            version: 8 as const,
            sources: {
                "online-source": {
                    type: "raster" as const,
                    tiles: [onlineUrl],
                    tileSize: 256,
                    attribution: "&copy; MapTiler &copy; OpenStreetMap",
                },
                "offline-source": {
                    type: "raster" as const,
                    tiles: [`/tiles/${mode}/{z}/{x}/{y}.png`],
                    tileSize: 256,
                },
            },
            layers: [
                {
                    id: "background",
                    type: "background" as const,
                    paint: { "background-color": "#f0f0f0" },
                },
                {
                    id: "online-layer",
                    type: "raster" as const,
                    source: "online-source",
                    paint: { "raster-opacity": 1 },
                },
                {
                    id: "offline-layer",
                    type: "raster" as const,
                    source: "offline-source",
                    paint: { "raster-opacity": 0 },
                },
            ],
        };
    }

    function switchStyle(mode: "normal" | "hybrid") {
        currentMode = mode;
        if (map) {
            map.setStyle(getStyle(mode));
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
            if (tab === "download") syncMinZoomWithCurrent();
        }
    }

    function toggleMarkerBottomPanel() {
        const now = Date.now();
        if (now - _lastMarkerToggleMs < 350) return;
        _lastMarkerToggleMs = now;
        showMarkerBottomPanel = !showMarkerBottomPanel;
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

    function applyOfflinePreference() {
        if (!map) return;
        const offlineOpacity = offlineMode ? 1 : 0;
        const onlineOpacity = offlineMode ? 0 : 1;
        if (map.getLayer("offline-layer")) {
            map.setPaintProperty(
                "offline-layer",
                "raster-opacity",
                offlineOpacity,
            );
        }
        if (map.getLayer("online-layer")) {
            map.setPaintProperty(
                "online-layer",
                "raster-opacity",
                onlineOpacity,
            );
        }
    }

    function toggleOnlineMode() {
        offlineMode = !offlineMode;
        applyOfflinePreference();
    }

    function toggleSatelliteMode() {
        switchStyle(currentMode === "hybrid" ? "normal" : "hybrid");
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
            const newBookmark = await DownloadRegion(
                currentMode,
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
    function createCustomMarkerElement(
        name: string,
        type: UserMarkerType,
    ): HTMLElement {
        const el = document.createElement("div");
        const label = document.createElement("div");
        label.className = "custom-marker-label";
        label.textContent = name;
        const pin = document.createElement("div");
        pin.className = "custom-marker-pin";

        if (type === "direction") {
            // No label — keeping the element as a pure 0×0 block ensures
            // MapLibre's anchor:"center" offset is exactly 0,0 at all angles.
            el.className = "custom-marker direction";
            el.appendChild(pin);
        } else {
            // Label on top, teardrop pin below; anchor:"bottom" maps to pin tip
            el.className = "custom-marker";
            el.appendChild(label);
            el.appendChild(pin);
        }
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
        markerType = null; // show type selector first
        editingMarkerId = null;
        newMarkerName = "";
        newMarkerLat = "";
        newMarkerLng = "";
        newMarkerAngle = "";
        pinPointMode = false;
    }

    function selectMarkerType(t: UserMarkerType) {
        markerType = t;
    }

    function cancelAddMarkerForm() {
        showAddMarkerForm = false;
        markerType = null;
        editingMarkerId = null;
        pinPointMode = false;
        newMarkerName = "";
        newMarkerLat = "";
        newMarkerLng = "";
        newMarkerAngle = "";
        removePinPointTempMarker();
        if (map) map.getCanvas().style.cursor = "";
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
        id: number,
        lat: number,
        lng: number,
        angle: number,
    ) {
        if (!map) return;
        const sourceId = `dir-line-${id}`;
        const layerId = `dir-line-layer-${id}`;
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
                paint: { "line-color": "#2563eb", "line-width": 2.5 },
            });
        } else {
            (map.getSource(sourceId) as maplibregl.GeoJSONSource).setData(
                geojson,
            );
        }
    }

    function updateDirectionLines() {
        if (!map) return;
        for (const m of userMarkers) {
            if (m.type === "direction" && m.angle !== undefined) {
                addDirectionLine(m.id, m.lat, m.lng, m.angle);
            }
        }
    }

    function removeDirectionLine(id: number) {
        if (!map) return;
        const layerId = `dir-line-layer-${id}`;
        const sourceId = `dir-line-${id}`;
        if (map.getLayer(layerId)) map.removeLayer(layerId);
        if (map.getSource(sourceId)) map.removeSource(sourceId);
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
        if (markerType === "direction") {
            const angle = parseFloat(newMarkerAngle);
            if (isNaN(angle)) {
                alert("Please enter a valid angle (0–360°).");
                return;
            }
        }
        if (!map) return;
        markerIdCounter++;
        const id = markerIdCounter;
        const name = newMarkerName.trim();
        const type = markerType ?? "point";
        const el = createCustomMarkerElement(name, type);
        // Direction marker: anchor at circle center so the line origin matches the dot.
        // Point marker: anchor at bottom so the teardrop tip touches the coordinate.
        const anchor = type === "direction" ? "center" : "bottom";
        const mapMarker = new maplibregl.Marker({ element: el, anchor })
            .setLngLat([lng, lat])
            .addTo(map);
        if (type === "direction") {
            const angle = ((parseFloat(newMarkerAngle) % 360) + 360) % 360;
            userMarkers = [
                ...userMarkers,
                { id, name, lat, lng, type, angle, mapMarker },
            ];
            addDirectionLine(id, lat, lng, angle);
        } else {
            userMarkers = [
                ...userMarkers,
                { id, name, lat, lng, type, mapMarker },
            ];
        }
        removePinPointTempMarker();
        cancelAddMarkerForm();
        saveMarkersToStorage();
    }

    function removeUserMarker(id: number) {
        const idx = userMarkers.findIndex((m) => m.id === id);
        if (idx !== -1) {
            userMarkers[idx].mapMarker.remove();
            if (userMarkers[idx].type === "direction") removeDirectionLine(id);
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
        markerType = m.type; // skip type selector, go straight to fields
        newMarkerName = m.name;
        newMarkerLat = String(m.lat);
        newMarkerLng = String(m.lng);
        newMarkerAngle = m.angle !== undefined ? String(m.angle) : "";
        pinPointMode = false;
        // Expand the list so the form is visible alongside it
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
        if (markerType === "direction") {
            const angle = parseFloat(newMarkerAngle);
            if (isNaN(angle)) {
                alert("Please enter a valid angle (0\u2013360\u00b0).");
                return;
            }
        }
        const idx = userMarkers.findIndex((m) => m.id === editingMarkerId);
        if (idx === -1) return;
        const existing = userMarkers[idx];
        // Remove the old map marker and direction line
        existing.mapMarker.remove();
        if (existing.type === "direction") removeDirectionLine(existing.id);
        // Recreate with updated values
        const name = newMarkerName.trim();
        const type = markerType ?? existing.type;
        const el = createCustomMarkerElement(name, type);
        const anchor = type === "direction" ? "center" : "bottom";
        const mapMarker = new maplibregl.Marker({ element: el, anchor })
            .setLngLat([lng, lat])
            .addTo(map);
        let updated: UserMarker;
        if (type === "direction") {
            const angle = ((parseFloat(newMarkerAngle) % 360) + 360) % 360;
            updated = { ...existing, name, lat, lng, type, angle, mapMarker };
            addDirectionLine(existing.id, lat, lng, angle);
        } else {
            updated = { ...existing, name, lat, lng, type, mapMarker };
        }
        userMarkers = userMarkers.map((m) =>
            m.id === existing.id ? updated : m,
        );
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
        map.setStyle(getStyle(currentMode));
    });

    onMount(async () => {
        // Fire config load without awaiting — the $effect above will update
        // the map style once the API key arrives.
        configStore.load();

        const lat = locationStore.data.latitude ?? -2.2;
        const lng = locationStore.data.longitude ?? 118;
        map = new maplibregl.Map({
            container: mapContainer,
            style: getStyle(currentMode),
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
        map.on("styledata", applyOfflinePreference);
        map.on("styledata", updateDFLine);
        map.on("styledata", updateDirectionLines);
        mapClickHandler = (e) => {
            if (pinPointMode) {
                newMarkerLat = e.lngLat.lat.toFixed(6);
                newMarkerLng = e.lngLat.lng.toFixed(6);
                if (!pinPointTempMarker) {
                    pinPointTempMarker = new maplibregl.Marker({
                        element: createPinPointTempElement(),
                        anchor: "bottom",
                        offset: [0, -16],
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
        applyOfflinePreference();
        unsubscribeDownloadEvents = EventsOn("download-status", (...payload) =>
            handleDownloadStatusEvent((payload?.[0] as DownloadEvent) || null),
        );
        fetchBookmarks();
        findMyLocation();
    });

    onDestroy(() => {
        if (noticeTimer) {
            clearTimeout(noticeTimer);
        }
        if (map) {
            map.off("zoom", updateCurrentZoom);
            map.off("zoomend", updateCurrentZoom);
            map.off("styledata", applyOfflinePreference);
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
            <button
                class="toolbar-btn satellite-btn"
                class:active-state={currentMode === "hybrid"}
                onclick={toggleSatelliteMode}
                aria-label="Toggle satellite view"
                title="Satellite"
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
                    <path d="M4 14l6 6" />
                    <path d="M9 15l-4 4" />
                    <path d="M15 9l4-4" />
                    <path d="M14 4l6 6" />
                    <path d="M8 13l3 3 5-5-3-3z" />
                </svg>
                Satellite
            </button>
            <button
                class="toolbar-btn online-btn hide-button"
                class:active-state={!offlineMode}
                onclick={toggleOnlineMode}
                aria-label="Toggle online tiles"
                title="Online"
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
                    <path d="M2 12a10 10 0 0 1 20 0" />
                    <path d="M6 12a6 6 0 0 1 12 0" />
                    <circle cx="12" cy="16" r="1.5" />
                </svg>
            </button>
            
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
                class="toolbar-btn download-btn-toggle hide-button"
                class:active-state={showDownloadPanel}
                disabled={isDownloading}
                aria-label="Open maps download"
                title="Maps Download"
                onclick={() => {
                    console.log("Maps Download clicked", {
                        isDownloading,
                        showDownloadPanel,
                    });
                    toggleDownloadPanel(downloadTab);
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

            <div class="toolbar-indicator hide-button" title="Zoom Value">Z |{currentZoom.toFixed(1)}</div>
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
                        onclick={() => (downloadTab = "bookmarks")}
                        >Saved Maps</button
                    >
                    <button
                        class="dl-tab"
                        class:active={downloadTab === "download"}
                        onclick={() => {
                            downloadTab = "download";
                            syncMinZoomWithCurrent();
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
                            {selectMode || selectionBounds
                                ? "Cancel Select"
                                : "Select Area"}
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
                                <button
                                    class="bookmark-btn"
                                    onclick={() => goToBookmark(b)}
                                >
                                    <span class="bookmark-title"
                                        >{b.title || "Untitled download"}</span
                                    >
                                    <span class="bookmark-meta">
                                        {b.style} | Zoom: {b.min_zoom}–{b.max_zoom}
                                        | [{b.center_lat.toFixed(4)}, {b.center_lng.toFixed(
                                            4,
                                        )}]
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
    </div>

    <!-- Marker Bottom Panel -->
    {#if showMarkerBottomPanel}
        <div class="marker-bottom-panel" class:pinpointing={pinPointMode}>
            <div class="marker-panel-header">
                <span class="marker-panel-title"
                    >Markers ({userMarkers.length})</span
                >
                <div class="marker-panel-actions">
                    {#if !showAddMarkerForm}
                        <button
                            class="marker-action-btn add"
                            onclick={openAddMarkerForm}
                        >
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
                    {#if markerType === null}
                        <!-- Step 1: choose marker type -->
                        <div class="marker-type-selector">
                            <span class="marker-type-label"
                                >Select marker type:</span
                            >
                            <div class="marker-type-btns">
                                <button
                                    class="marker-type-btn point"
                                    onclick={() => selectMarkerType("point")}
                                >
                                    <span class="mtype-icon">📍</span>
                                    <span class="mtype-name">Point Marker</span>
                                    <span class="mtype-desc"
                                        >Marks a place on the map</span
                                    >
                                </button>
                                <button
                                    class="marker-type-btn direction"
                                    onclick={() =>
                                        selectMarkerType("direction")}
                                >
                                    <span class="mtype-icon">🧭</span>
                                    <span class="mtype-name"
                                        >Direction Marker</span
                                    >
                                    <span class="mtype-desc"
                                        >Marks a place with a direction line</span
                                    >
                                </button>
                            </div>
                            <button
                                class="marker-action-btn cancel"
                                style="align-self:flex-end"
                                onclick={cancelAddMarkerForm}>Cancel</button
                            >
                        </div>
                    {:else}
                        <!-- Step 2: fill in the form -->
                        <div class="marker-type-chip {markerType}">
                            {editingMarkerId !== null
                                ? "✏️ Edit — "
                                : ""}{markerType === "point"
                                ? "📍 Point Marker"
                                : "🧭 Direction Marker"}
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
                        {#if markerType === "direction"}
                            <label class="angle-label">
                                Direction angle (0–360°)
                                <input
                                    class="marker-input"
                                    type="number"
                                    placeholder="e.g. 45"
                                    min="0"
                                    max="360"
                                    step="1"
                                    style="margin-top:4px"
                                    bind:value={newMarkerAngle}
                                />
                            </label>
                        {/if}
                        {#if pinPointMode}
                            <div class="pinpoint-hint">
                                Click anywhere on the map to set coordinates
                            </div>
                        {/if}
                        <div class="form-actions">
                            {#if editingMarkerId !== null}
                                <button
                                    class="marker-action-btn add"
                                    onclick={confirmEditMarker}>Save</button
                                >
                            {:else}
                                <button
                                    class="marker-action-btn add"
                                    onclick={confirmAddMarker}>Add</button
                                >
                            {/if}
                            <button
                                class="marker-action-btn cancel"
                                onclick={cancelAddMarkerForm}>Cancel</button
                            >
                        </div>
                    {/if}
                </div>
            {/if}

            {#if showMarkerPanel}
                <div class="marker-list-scroll">
                    {#if userMarkers.length === 0}
                        <div class="marker-empty">No markers added yet.</div>
                    {:else}
                        {#each userMarkers as m}
                            <div class="marker-list-item">
                                <button
                                    class="marker-name-btn"
                                    onclick={() => flyToMarker(m)}
                                >
                                    <span class="marker-list-icon"
                                        >{m.type === "direction"
                                            ? "🧭"
                                            : "📍"}</span
                                    >
                                    <span class="marker-list-name"
                                        >{m.name}</span
                                    >
                                    <span class="marker-list-coords">
                                        {m.lat.toFixed(4)}, {m.lng.toFixed(4)}
                                        {#if m.type === "direction" && m.angle !== undefined}
                                            &nbsp;· {m.angle.toFixed(0)}°
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
                                    aria-label="Remove marker {m.name}"
                                    >✕</button
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
        position: relative;
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
        width: 280px;
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
        flex-wrap: wrap;
        align-items: center;
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

    .toolbar > * {
        width: auto;
        min-width: 0;
    }

    .toolbar-btn,
    .toolbar-indicator,
    .toolbar-checkbox {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: 6px;
        height: 34px;
        padding: 0 10px;
        border-radius: 10px;
        background: #ffffff;
        border: 1px solid rgba(148, 163, 184, 0.45);
        color: #0f172a;
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

    .toolbar-btn:hover,
    .toolbar-checkbox:hover {
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

    .toolbar-btn.satellite-btn.active-state {
        background: #6366f1;
        border-color: #4f46e5;
        color: #ffffff;
        box-shadow: 0 6px 14px rgba(79, 70, 229, 0.28);
    }

    .toolbar-btn.online-btn.active-state {
        background: #10b981;
        border-color: #059669;
        color: #ffffff;
        box-shadow: 0 6px 14px rgba(5, 150, 105, 0.28);
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

    .toolbar-checkbox {
        cursor: pointer;
        user-select: none;
        padding: 0 10px;
    }

    .toolbar-checkbox.active {
        color: #0f172a;
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
    }

    .marker-action-btn.cancel {
        background: #e2e8f0;
        color: #475569;
    }

    .marker-action-btn.cancel:hover {
        background: #cbd5e1;
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
        gap: 8px;
    }

    .marker-list-scroll {
        overflow-y: auto;
        max-height: 36vh;
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
        position: relative;
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

    /* Custom map marker pin — point (red teardrop), label on top */
    :global(.custom-marker) {
        display: flex;
        flex-direction: column;
        align-items: center;
        cursor: pointer;
    }

    /* Label sits above the pin for point markers */
    :global(.custom-marker:not(.direction) .custom-marker-label) {
        order: -1;
        margin-bottom: 3px;
        margin-top: 0;
    }

    :global(.custom-marker-pin) {
        width: 20px;
        height: 20px;
        border-radius: 50% 50% 50% 0;
        background: #ef4444;
        border: 2px solid #ffffff;
        box-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
        transform: rotate(-45deg);
        flex-shrink: 0;
    }

    /* Direction marker — fixed 20×20 so MapLibre's anchor:"center" centres it
       exactly on the coordinate.  No label; circle fills the element. */
    :global(.custom-marker.direction) {
        width: 20px;
        height: 20px;
        display: block;
        cursor: pointer;
    }

    :global(.custom-marker.direction .custom-marker-pin) {
        width: 20px;
        height: 20px;
        background: #16a34a;
        border-radius: 50%;
        transform: none;
        border: 2px solid #ffffff;
        box-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
    }

    :global(.custom-marker-label) {
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
    /* Marker type selector */
    .marker-type-selector {
        display: flex;
        flex-direction: column;
        gap: 8px;
        padding-top: 8px;
    }

    .marker-type-label {
        font-size: 12px;
        font-weight: 600;
        color: #475569;
    }

    .marker-type-btns {
        display: flex;
        gap: 8px;
    }

    .marker-type-btn {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 3px;
        padding: 10px 8px;
        border-radius: 8px;
        border: 2px solid #e2e8f0;
        background: #f8fafc;
        cursor: pointer;
        transition:
            border-color 0.15s ease,
            background 0.15s ease;
    }

    .marker-type-btn:hover {
        background: #f0f9ff;
    }

    .marker-type-btn.point:hover {
        border-color: #ef4444;
    }

    .marker-type-btn.direction:hover {
        border-color: #f97316;
    }

    .mtype-icon {
        font-size: 20px;
    }

    .mtype-name {
        font-size: 12px;
        font-weight: 700;
        color: #0f172a;
    }

    .mtype-desc {
        font-size: 10px;
        color: #64748b;
        text-align: center;
    }

    .marker-type-chip {
        display: inline-block;
        font-size: 11px;
        font-weight: 700;
        padding: 2px 8px;
        border-radius: 999px;
        margin-top: 8px;
        align-self: flex-start;
    }

    .marker-type-chip.point {
        background: #fee2e2;
        color: #b91c1c;
    }

    .marker-type-chip.direction {
        background: #ffedd5;
        color: #c2410c;
    }

    .angle-label {
        font-size: 12px;
        color: #475569;
        display: flex;
        flex-direction: column;
    }

    .hide-button {
        display: none;
    }
    /* ---- End Marker Bottom Panel ---- */
</style>
