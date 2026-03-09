<script lang="ts">
  import { onMount } from "svelte";
  import type { Component } from "svelte";
  import { locationStore } from "./lib/store/locationStore.svelte";

  // Both heavy components are lazy-loaded so the window opens instantly.
  // Maps.svelte brings in ~1 MB of maplibre-gl.
  // DFPanel.svelte kicks off all the network polling to 192.168.17.17.
  // Neither blocks the initial paint.
  let MapsComponent: Component | null = $state(null);
  let DFPanelComponent: Component | null = $state(null);
  let isMapEnabled = $state(false);

  function refreshApp() {
    window.location.reload();
  }

  onMount(async () => {
    locationStore.fetchGPSExternal();

    // Load Maps first — it's visual and has no blocking network calls.
    const { default: Maps } = await import("./lib/component/Maps.svelte");
    MapsComponent = Maps;

    // Load DFPanel after Maps is mounted. Its network polling to the
    // external device starts only once this component is created.
    const { default: DFPanel } = await import("./lib/component/DFPanel.svelte");
    DFPanelComponent = DFPanel;
  });
</script>

<main style="background-color: whitesmoke;">
  <div class="top-controls">
    <label class="map-toggle" class:active={isMapEnabled}>
      <input type="checkbox" bind:checked={isMapEnabled} />
      <span>Map</span>
    </label>
    <button class="refresh-btn" onclick={refreshApp} aria-label="Refresh page">
      Refresh
    </button>
  </div>

  {#if isMapEnabled && MapsComponent}
    <MapsComponent />
  {/if}

  {#if DFPanelComponent}
    <DFPanelComponent />
  {:else}
    <div class="dfpanel-loading">Loading…</div>
  {/if}
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }

  .map-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 27px;
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
    cursor: pointer;
    user-select: none;
  }

  .top-controls {
    position: fixed;
    top: 2px;
    right: 8px;
    z-index: 13000;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .map-toggle:hover {
    background: #f1f5f9;
    border-color: rgba(59, 130, 246, 0.4);
    box-shadow: 0 4px 10px rgba(37, 99, 235, 0.12);
  }

  .map-toggle.active {
    background: #e7f9ef;
    border-color: #34d399;
    color: #065f46;
  }

  .map-toggle input {
    width: 14px;
    height: 14px;
    accent-color: #0ea5e9;
    cursor: pointer;
  }

  .map-toggle span {
    pointer-events: none;
    font-size: 13px;
    font-weight: 600;
  }

  .refresh-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 27px;
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
    cursor: pointer;
    user-select: none;
    font-size: 13px;
    font-weight: 600;
  }

  .refresh-btn:hover {
    background: #f1f5f9;
    border-color: rgba(59, 130, 246, 0.4);
    box-shadow: 0 4px 10px rgba(37, 99, 235, 0.12);
  }

  .dfpanel-loading {
    width: 120px;
    padding: 6px 10px;
    background: rgba(4, 61, 15, 0.6);
    color: white;
    font-size: 13px;
    border-radius: 6px;
    box-shadow: 0 3px 8px rgba(0, 0, 0, 0.2);
    position: fixed;
    top: 46px;
    right: 8px;
  }
</style>
