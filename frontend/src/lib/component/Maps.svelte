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
    let currentMode = $state<"normal" | "hybrid">("normal"); // 'normal' or 'hybrid'
    let showDownloadMenu = $state(false);
    let showBookmarkList = $state(false);
    let completionNotice = $state("");
    let downloadTitle = $state("");
    let noticeTimer: ReturnType<typeof setTimeout> | null = null;
    let unsubscribeDownloadEvents: (() => void) | null = null;
    let currentZoom = $state(14);
    let offlineMode = $state(false);
    let isClearing = $state(false);
    let downloadLocked = $state(false);

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
    const API_KEY = "aUOEn1bA48mz3xc3pL4N";
    //! buat key bisa diganti di runtime nanti

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
        if (next) {
            showBookmarkList = false;
            syncMinZoomWithCurrent();
        } else {
            cancelSelection();
        }
    }

    function toggleBookmarkPanel() {
        const next = !showBookmarkList;
        showBookmarkList = next;
        if (next && showDownloadMenu) {
            showDownloadMenu = false;
            cancelSelection();
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

    function handleOnlineToggle(event: Event) {
        const target = event.target as HTMLInputElement | null;
        if (!target) return;
        offlineMode = !target.checked;
        applyOfflinePreference();
    }

    function handleSatelliteToggle(event: Event) {
        const target = event.target as HTMLInputElement | null;
        if (!target) return;
        switchStyle(target.checked ? "hybrid" : "normal");
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

    onMount(() => {
        map = new maplibregl.Map({
            container: mapContainer,
            style: getStyle(currentMode),
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
        unsubscribeDownloadEvents = EventsOn("download-status", (...payload) =>
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
        <div class="toolbar">
            <button class="toolbar-btn" onclick={toggleBookmarkPanel}>
                <!-- {showBookmarkList ? "Hide Downloads" : "Show Downloads"} -->
                Downloads List
            </button>
            <button
                class="toolbar-btn"
                class:active-state={showDownloadMenu}
                disabled={isDownloading}
                onclick={toggleDownloadMenu}
            >
                Download Maps
            </button>
            <label class="toolbar-checkbox online" class:active={!offlineMode}>
                <input
                    type="checkbox"
                    checked={!offlineMode}
                    onchange={handleOnlineToggle}
                />
                <span>Online</span>
            </label>
            <label
                class="toolbar-checkbox satellite"
                class:active={currentMode === "hybrid"}
            >
                <input
                    type="checkbox"
                    checked={currentMode === "hybrid"}
                    onchange={handleSatelliteToggle}
                />
                <span>Satellite</span>
            </label>
            <div class="toolbar-indicator">Zoom {currentZoom.toFixed(2)}</div>
        </div>
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
            </div>
        {/if}
    </div>

    {#if showBookmarkList}
        <div class="bookmark-list">
            {#each bookmarks as b}
                <button class="bookmark-btn" onclick={() => goToBookmark(b)}>
                    <span class="bookmark-title"
                        >{b.title || "Untitled download"}</span
                    >
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
        top: 46px;
        z-index: 5;
        display: flex;
        flex-direction: column;
        gap: 8px;
        max-height: calc(80vh - 80px);
        overflow-y: auto;
        width: 280px;
        padding: 12px;
        background: rgba(255, 255, 255, 0.95);
        border-radius: 8px;
        border: 1px solid rgba(0, 0, 0, 0.08);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
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
        top: 6px;
        left: 8px;
        z-index: 10;
        display: flex;
        flex-direction: column;
        gap: 8px;
        align-items: flex-start;
        pointer-events: none;
    }

    .controls > * {
        pointer-events: auto;
    }

    .toolbar {
        display: inline-flex;
        flex-direction: row;
        align-items: stretch;
        gap: 0;
        flex-wrap: nowrap;
        background: rgba(255, 255, 255, 0.95);
        border-radius: 999px;
        box-shadow: 0 3px 8px rgba(0, 0, 0, 0.2);
        border: 1px solid rgba(0, 0, 0, 0.1);
        overflow: hidden;
    }

    .toolbar-btn,
    .toolbar-indicator {
        border: none;
        background: transparent;
        padding: 8px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: 6px;
        cursor: pointer;
        color: #0f172a;
        transition:
            background 0.15s ease,
            color 0.15s ease;
    }

    .toolbar > * + * {
        border-left: 1px solid rgba(15, 23, 42, 0.1);
    }

    .toolbar-btn:hover {
        background: rgba(15, 23, 42, 0.06);
    }

    .toolbar-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .toolbar-btn.active-state {
        background: rgba(37, 99, 235, 0.15);
        color: #1d4ed8;
    }

    .toolbar-checkbox {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 0 8px;
        cursor: pointer;
        color: #0f172a;
        user-select: none;
        background: transparent;
    }

    .toolbar-checkbox input {
        width: 16px;
        height: 16px;
        accent-color: #22c55e;
        cursor: pointer;
    }

    .toolbar-checkbox span {
        pointer-events: none;
    }

    .toolbar-checkbox.active {
        color: white;
    }

    .toolbar-checkbox.online.active {
        background: #22c55e;
    }

    .toolbar-checkbox.satellite input {
        accent-color: #6366f1;
    }

    .toolbar-checkbox.satellite.active {
        background: #6366f1;
    }

    .toolbar-indicator {
        padding: 0 16px;
        font-size: 13px;
        color: #1e293b;
        min-width: 110px;
        cursor: default;
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
        background: rgba(255, 255, 255, 0.95);
        border-radius: 6px;
        padding: 10px;
        box-shadow: 0 3px 6px rgba(0, 0, 0, 0.15);
        display: flex;
        flex-direction: column;
        gap: 10px;
        width: 200px;
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
</style>
