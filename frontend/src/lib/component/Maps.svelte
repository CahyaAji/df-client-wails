<script lang="ts">
    import maplibregl from "maplibre-gl";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { onMount, onDestroy } from "svelte";

    import {
        ClearDownloads,
        DownloadRegion,
        ListBookmarks,
    } from "../../../wailsjs/go/main/App";
    import { EventsOn } from "../../../wailsjs/runtime/runtime";
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
            map.setStyle(getStyle(b.style as "normal" | "hybrid"));
            map.setCenter([b.center_lng, b.center_lat]);
            map.setZoom(b.min_zoom);
        }
    }

    let mapContainer: HTMLElement;
    let map: maplibregl.Map;

    // Svelte 5 State
    let isDownloading = $state(false);
    let downloadStatus = $state("Ready");
    let currentMode = $state("normal"); // 'normal' or 'hybrid'
    let showDownloadMenu = $state(false);
    let showBookmarkList = $state(false);
    let completionNotice = $state("");
    let downloadTitle = $state("");
    let noticeTimer: ReturnType<typeof setTimeout> | null = null;
    let unsubscribeDownloadEvents: (() => void) | null = null;
    let currentZoom = $state(14);
    let offlineMode = $state(false);
    let isClearing = $state(false);

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

    const API_KEY = "fB2eDjoDg2nlel5Kw6ym";

    function getStyle(mode: "normal" | "hybrid") {
        const isHybrid = mode === "hybrid";

        const onlineUrl = isHybrid
            ? `https://api.maptiler.com/maps/hybrid/{z}/{x}/{y}.jpg?key=${API_KEY}`
            : `https://api.maptiler.com/maps/openstreetmap/{z}/{x}/{y}.jpg?key=${API_KEY}`;

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

    function toggleDownloadMenu() {
        const next = !showDownloadMenu;
        showDownloadMenu = next;
        if (!next) {
            cancelSelection();
        } else {
            syncMinZoomWithCurrent();
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

    function applyOfflinePreference() {
        if (!map) return;
        const offlineOpacity = offlineMode ? 1 : 0;
        const onlineOpacity = offlineMode ? 0 : 1;
        if (map.getLayer("offline-layer")) {
            map.setPaintProperty("offline-layer", "raster-opacity", offlineOpacity);
        }
        if (map.getLayer("online-layer")) {
            map.setPaintProperty("online-layer", "raster-opacity", onlineOpacity);
        }
    }

    function toggleOfflineMode() {
        offlineMode = !offlineMode;
        applyOfflinePreference();
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
        }
    }

    function handleDownloadStatusEvent(eventData: DownloadEvent) {
        if (!eventData) return;
        const title = eventData.title?.trim() || "Download";
        if (eventData.status === "complete") {
            const message = eventData.message || `${title} ready`;
            downloadStatus = message;
            showCompletion(message);
            return;
        }
        if (eventData.status === "error") {
            const message = eventData.message || `${title} failed`;
            downloadStatus = message;
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

    onMount(() => {
        map = new maplibregl.Map({
            container: mapContainer,
            style: getStyle("normal"),
            center: [110.44053927286228, -7.777395993083473], // Yogyakarta
            zoom: 14,
        });
        map.addControl(new maplibregl.ScaleControl(), "bottom-left");
        map.addControl(new maplibregl.NavigationControl(), "bottom-left");
        updateCurrentZoom();
        syncMinZoomWithCurrent();
        map.on("zoom", updateCurrentZoom);
        map.on("zoomend", updateCurrentZoom);
        map.on("styledata", applyOfflinePreference);
        applyOfflinePreference();
        unsubscribeDownloadEvents = EventsOn(
            "download-status",
            (...payload) =>
                handleDownloadStatusEvent((payload?.[0] as DownloadEvent) || null),
        );
        fetchBookmarks();
    });

    onDestroy(() => {
        if (noticeTimer) {
            clearTimeout(noticeTimer);
        }
        if (map) {
            map.off("zoom", updateCurrentZoom);
            map.off("zoomend", updateCurrentZoom);
            map.off("styledata", applyOfflinePreference);
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
        <div class="btn-group">
            <button
                class:active={currentMode === "normal"}
                onclick={() => switchStyle("normal")}>Normal View</button
            >
            <button
                class:active={currentMode === "hybrid"}
                onclick={() => switchStyle("hybrid")}>Hybrid View</button
            >
        </div>
        <div class="zoom-indicator">Zoom: {currentZoom.toFixed(2)}</div>
        <button
            class="download-btn"
            disabled={isDownloading}
            onclick={toggleDownloadMenu}
        >
            {showDownloadMenu ? "Hide Download Menu" : "Download"}
        </button>
        <label class="mode-toggle" class:offline={offlineMode}>
            <input type="checkbox" checked={!offlineMode} onchange={toggleOfflineMode} />
            <span>{offlineMode ? "Offline Maps" : "Online Maps"}</span>
        </label>
        <button
            class="download-btn outline"
            onclick={() => (showBookmarkList = !showBookmarkList)}
        >
            {showBookmarkList ? "Hide Downloads" : "Show Downloads"}
        </button>
        {#if completionNotice}
            <div class="notice">{completionNotice}</div>
        {/if}
        {#if showDownloadMenu}
            <div class="download-menu">
                <div class="select-row">
                    <button
                        class="select-btn"
                        class:active={selectMode}
                        onclick={selectMode || selectionBounds
                            ? cancelSelection
                            : beginSelection}
                    >
                        {selectMode || selectionBounds
                            ? "Cancel Selection"
                            : "Select Area"}
                    </button>
                    {#if selectionBounds}
                        <div class="bounds-info">
                            N {selectionBounds.north.toFixed(2)} deg | S {selectionBounds.south.toFixed(
                                2,
                            )} deg<br />E {selectionBounds.east.toFixed(2)} deg |
                            W {selectionBounds.west.toFixed(2)} deg
                        </div>
                    {/if}
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
                        !titleInputValid()}
                    onclick={handleCustomDownload}
                >
                    {isDownloading ? "Downloading..." : "Download"}
                </button>
                <div class="status-row">
                    <span class="status">{downloadStatus}</span>
                </div>
                <button
                    class="download-btn danger"
                    disabled={isClearing}
                    onclick={clearAllDownloads}
                >
                    {isClearing ? "Clearing..." : "Delete All Downloads"}
                </button>
            </div>
        {/if}
    </div>

    {#if showBookmarkList}
        <div class="bookmark-list">
            {#each bookmarks as b}
                <button class="bookmark-btn" onclick={() => goToBookmark(b)}>
                    <span class="bookmark-title">{b.title || "Untitled download"}</span>
                    <span class="bookmark-meta">
                        {b.style} | Zoom: {b.min_zoom}-{b.max_zoom} | Center: [{b.center_lat.toFixed(
                            4,
                        )}, {b.center_lng.toFixed(4)}]
                    </span>
                </button>
            {/each}
        </div>
    {/if}
    <div
        class="map-container"
        class:selecting={selectMode}
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
</div>

<style>
    .bookmark-list {
        position: absolute;
        left: 10px;
        top: 10px;
        z-index: 20;
        display: flex;
        flex-direction: column;
        gap: 6px;
        max-height: 80vh;
        overflow-y: auto;
    }
    .bookmark-btn {
        background: #fffbe6;
        border: 1px solid #e0c97f;
        border-radius: 4px;
        padding: 6px 10px;
        font-size: 13px;
        cursor: pointer;
        text-align: left;
        transition: background 0.2s;
    }
    .bookmark-title {
        font-weight: 600;
        display: block;
    }
    .bookmark-meta {
        font-size: 12px;
        color: #555;
        display: block;
        margin-top: 2px;
    }
    .bookmark-btn:hover {
        background: #ffe066;
    }
    .map-layout {
        height: 100%;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        border: 1px solid #ccc;
        position: relative;
    }

    .map-container {
        flex-grow: 1;
        width: 100%;
        height: 100%;
    }

    .map-container.selecting {
        touch-action: none;
        cursor: default;
    }

    /* Floating Controls */
    .controls {
        position: absolute;
        top: 10px;
        right: 10px;
        z-index: 10;
        display: flex;
        flex-direction: column;
        gap: 10px;
        align-items: flex-end;
    }

    .zoom-indicator {
        background: white;
        border-radius: 4px;
        padding: 4px 8px;
        font-size: 13px;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.15);
        font-weight: 600;
    }

    .btn-group {
        background: white;
        border-radius: 4px;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
        overflow: hidden;
        display: flex;
    }

    .btn-group button {
        border: none;
        background: white;
        padding: 8px 12px;
        cursor: pointer;
        font-size: 14px;
        border-right: 1px solid #eee;
    }

    .btn-group button:last-child {
        border-right: none;
    }

    .btn-group button.active {
        background: #eee;
        font-weight: bold;
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
        background: #28a745;
    }

    .download-btn.danger {
        background: #dc2626;
    }

    .download-btn.danger:disabled {
        background: #fca5a5;
    }

    .download-btn.outline {
        background: white;
        color: #007bff;
        border: 1px solid #007bff;
    }

    .mode-toggle {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        border-radius: 999px;
        padding: 6px 14px;
        font-weight: 600;
        cursor: pointer;
        user-select: none;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.15);
        color: #14532d;
        border: 1px solid #7dd3ae;
        background: #e2f4e9;
    }

    .mode-toggle input {
        accent-color: #16a34a;
        width: 16px;
        height: 16px;
    }

    .mode-toggle.offline {
        background: #f1f5f9;
        color: #475569;
        border-color: #cbd5f5;
    }

    .download-menu {
        background: rgba(255, 255, 255, 0.95);
        border-radius: 6px;
        padding: 10px;
        box-shadow: 0 3px 6px rgba(0, 0, 0, 0.15);
        display: flex;
        flex-direction: column;
        gap: 10px;
        width: 250px;
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
    }

    .select-btn.active {
        background: #007bff;
        color: white;
    }

    .bounds-info {
        font-size: 12px;
        color: #333;
        text-align: right;
        line-height: 1.3;
    }

    .zoom-inputs {
        display: flex;
        gap: 10px;
    }

    .zoom-inputs label {
        display: flex;
        flex-direction: column;
        font-size: 12px;
        color: #555;
        flex: 1;
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

    .status-row {
        width: 100%;
        display: flex;
        justify-content: flex-end;
    }

    .status {
        background: rgba(0, 0, 0, 0.7);
        color: white;
        padding: 4px 8px;
        border-radius: 4px;
        font-size: 12px;
        margin-right: 5px;
    }

    .map-container {
        position: relative;
    }

    .selection-overlay {
        position: absolute;
        border: 2px dashed #007bff;
        background: rgba(0, 123, 255, 0.15);
        pointer-events: none;
        z-index: 5;
    }
</style>
