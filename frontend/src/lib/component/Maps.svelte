<script lang="ts">
    import maplibregl from "maplibre-gl";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { onMount } from "svelte";

    let mapContainer: HTMLElement;
    let map: maplibregl.Map;

    const MAP_STYLES = {
        normal: "https://api.maptiler.com/maps/openstreetmap/style.json?key=fB2eDjoDg2nlel5Kw6ym",
        hybrid: "https://api.maptiler.com/maps/hybrid/style.json?key=aUOEn1bA48mz3xc3pL4N",
    };

    onMount(() => {
        map = new maplibregl.Map({
            container: mapContainer,
            style: MAP_STYLES.normal,
            center: [110.44053927286228, -7.777395993083473],
            zoom: 14,
        });
        map.addControl(new maplibregl.NavigationControl(), "top-left");

        return () => {
            map.remove();
        };
    });
</script>

<div class="map-layout">
    <!-- <div class="map-buttons">
        <button
            class:active={$settings.mapStyle === "normal"}
            onclick={() => switchStyle("normal")}>Normal</button
        >
        <button
            class:active={$settings.mapStyle === "hybrid"}
            onclick={() => switchStyle("hybrid")}>Satellite</button
        >
    </div> -->
    <div class="map-container" bind:this={mapContainer}></div>
</div>

<style>
    .map-layout {
        height: 100%;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        border: 1px solid #ccc;
    }

    .map-container {
        flex-grow: 1;
        width: 100%;
        height: 100%;
        position: relative;
    }
</style>
