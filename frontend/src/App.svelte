<script lang="ts">
    import { on } from "svelte/events";
  import DFPanel from "./lib/component/DFPanel.svelte";
  // import logo from './assets/images/logo-universal.png'
  // import {Greet} from '../wailsjs/go/main/App.js'
  import Maps from "./lib/component/Maps.svelte";
    import { onMount } from "svelte";
    import { locationStore } from "./lib/store/locationStore.svelte";

  onMount(() => {
    locationStore.fetchGPS();
  });
</script>

<main>
  <Maps />
  <DFPanel />
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
  }
</style>
