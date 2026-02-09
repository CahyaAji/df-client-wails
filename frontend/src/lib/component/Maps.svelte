<script lang="ts">
    import maplibregl from "maplibre-gl";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { onMount, onDestroy } from "svelte";

    // Import the Go backend function
    // Make sure you have run 'wails dev' so this file is generated
    import {
        DownloadRegion,
        ListBookmarks,
    } from "../../../wailsjs/go/main/App";
    // Bookmarks state
    let bookmarks: Array<{
        id: number;
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

    // Your API Key
    const API_KEY = "fB2eDjoDg2nlel5Kw6ym"; // Replace with yours if different

    // We build the Style JSON manually to support both Offline (Go) and Online (MapTiler)
    function getStyle(mode: "normal" | "hybrid") {
        const isHybrid = mode === "hybrid";

        // Online Source (MapTiler)
        const onlineUrl = isHybrid
            ? `https://api.maptiler.com/maps/hybrid/{z}/{x}/{y}.jpg?key=${API_KEY}`
            : `https://api.maptiler.com/maps/openstreetmap/{z}/{x}/{y}.jpg?key=${API_KEY}`;

        return {
            version: 8 as const,
            sources: {
                // 1. Online Source (Fallback)
                "online-source": {
                    type: "raster" as const,
                    tiles: [onlineUrl],
                    tileSize: 256,
                    attribution: "&copy; MapTiler &copy; OpenStreetMap",
                },
                // 2. Offline Source (Local Go Server)
                // Wails serves this from your SQLite database
                "offline-source": {
                    type: "raster" as const,
                    tiles: [`/tiles/${mode}/{z}/{x}/{y}.png`], // e.g., /tiles/normal/12/3/4.png
                    tileSize: 256,
                },
            },
            layers: [
                {
                    id: "background",
                    type: "background" as const,
                    paint: { "background-color": "#f0f0f0" },
                },
                // Layer 1: Online (Bottom)
                {
                    id: "online-layer",
                    type: "raster" as const,
                    source: "online-source",
                    paint: { "raster-opacity": 1 },
                },
                // Layer 2: Offline (Top)
                // If offline tile exists, it covers the online one.
                // If missing (404), MapLibre sees through to the Online layer.
                {
                    id: "offline-layer",
                    type: "raster" as const,
                    source: "offline-source",
                    paint: { "raster-opacity": 1 },
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

    async function handleDownload() {
        if (!map) return;

        isDownloading = true;
        downloadStatus = "Preparing...";

        // 1. Get Bounds from current view
        const bounds = map.getBounds();
        const north = bounds.getNorth();
        const south = bounds.getSouth();
        const east = bounds.getEast();
        const west = bounds.getWest();

        // 2. Set Constraints (Don't let users download the whole world!)
        const minZ = 12;
        const maxZ = 14; // Be careful going higher than 14 (file size explodes)

        try {
            // Call Go Backend with mode as first argument
            downloadStatus = await DownloadRegion(
                currentMode,
                minZ,
                maxZ,
                north,
                south,
                east,
                west,
            );
            setTimeout(() => {
                isDownloading = false;
                alert(`Download Finished! Saved to database.`);
            }, 1000);
        } catch (err) {
            console.error(err);
            downloadStatus = "Error occurred";
            isDownloading = false;
        }
    }

    onMount(() => {
        map = new maplibregl.Map({
            container: mapContainer,
            style: getStyle("normal"),
            center: [110.44053927286228, -7.777395993083473], // Yogyakarta
            zoom: 14,
        });
        map.addControl(new maplibregl.NavigationControl(), "top-left");
        map.addControl(new maplibregl.ScaleControl(), "bottom-left");
        fetchBookmarks();
    });

    onDestroy(() => {
        if (map) map.remove();
    });
</script>

<div class="map-layout">
    <div class="controls">
        <div class="btn-group">
            <button
                class="download-btn"
                disabled={isDownloading}
                on:click={handleDownload}
            >
                {isDownloading ? "Downloading..." : "Download Area"}
            </button>
        </div>
    </div>

    <div class="bookmark-list">
        {#each bookmarks as b}
            <button class="bookmark-btn" on:click={() => goToBookmark(b)}>
                {b.style} | Zoom: {b.min_zoom}-{b.max_zoom} | Center: [{b.center_lat.toFixed(
                    4,
                )}, {b.center_lng.toFixed(4)}]
            </button>
        {/each}
    </div>
    <div class="map-container" bind:this={mapContainer}></div>
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

    .status {
        background: rgba(0, 0, 0, 0.7);
        color: white;
        padding: 4px 8px;
        border-radius: 4px;
        font-size: 12px;
        margin-right: 5px;
    }

    .download-section {
        display: flex;
        align-items: center;
    }
</style>
