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

  onMount(async () => {
    locationStore.fetchGPS();

    // Load Maps first — it's visual and has no blocking network calls.
    const { default: Maps } = await import("./lib/component/Maps.svelte");
    MapsComponent = Maps;

    // Load DFPanel after Maps is mounted. Its network polling to the
    // external device starts only once this component is created.
    const { default: DFPanel } = await import("./lib/component/DFPanel.svelte");
    DFPanelComponent = DFPanel;
  });
</script>

<main>
  {#if MapsComponent}
    <MapsComponent />
  {:else}
    <div class="map-placeholder"></div>
  {/if}

  {#if DFPanelComponent}
    <DFPanelComponent />
  {:else}
    <div class="dfpanel-placeholder">
      <div class="dfpanel-loading">Loading…</div>
    </div>
  {/if}
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
  }

  .map-placeholder {
    flex: 1;
    background: #f0f0f0;
  }

  /* Mirrors DFPanel's absolute position so there's no layout shift */
  .dfpanel-placeholder {
    position: absolute;
    top: 50px;
    right: 8px;
    width: 0;
    height: 0;
  }

  .dfpanel-loading {
    width: 120px;
    padding: 6px 10px;
    background: rgba(4, 61, 15, 0.6);
    color: white;
    font-size: 13px;
    border-radius: 6px;
    box-shadow: 0 3px 8px rgba(0, 0, 0, 0.2);
  }
</style>
