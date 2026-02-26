<script>
  import { compassStore } from "../store/compassStore.svelte";
  // import { configStore } from "../store/configStore.svelte";
  import { signalState } from "../store/signalState.svelte";

  let inputOffset = $state(0);
  let errorMessage = $state("");
  let isSaving = $state(false); // Add this flag

  let compassOffset = $derived(signalState.compassOffset);

  async function handleSetOffset() {
    if (inputOffset === null || inputOffset === undefined) {
      console.warn("offset value is empty");
      errorMessage = "Offset tidak boleh kosong";
      setTimeout(() => {
        errorMessage = "";
      }, 1500);
      return;
    }

    if (inputOffset < -360 || inputOffset > 360) {
      errorMessage = "Offset harus diantara -360 sampai 360";
      setTimeout(() => {
        errorMessage = "";
      }, 1500);
      return;
    }

    signalState.setCompassOffset(inputOffset);

    // isSaving = true; // Set flag before save
    // const result = await configStore.setCompassOffset(inputOffset);
    // isSaving = false; // Clear flag after save
    
    // if (result.success) {
    //   console.log("Compass offset updated successfully: " + inputOffset);
    // } else {
    //   console.error("Failed to update compass offset:", result.error);
    //   errorMessage = "Error: Gagal mengatur offset";
    //   setTimeout(() => {
    //     errorMessage = "";
    //   }, 1500);
    // }
  }

  $effect(() => {
    // Only sync from store when NOT saving
    if (!isSaving) {
      inputOffset = compassOffset;
    }
  });
</script>

<div class="container">
  <div class="compass-display">
    <div class="compass">
      {#if compassStore.data !== undefined && compassStore.data !== null}
        <div
          class="rotating-circle"
          style="transform: rotate({360 -
            compassStore.data -
            compassOffset}deg); transition: transform 0.2s;"
        >
          N
        </div>
      {/if}
      <div class="circle">
        <div class="arrow"></div>
      </div>
    </div>
    {#if compassStore.data && compassStore.data !== undefined && compassStore.data !== null}
      <div class="cmps-value">
        {(Math.round(compassStore.data + compassOffset) + 360) % 360}°
      </div>
    {:else}
      <div class="cmps-value">--</div>
    {/if}
  </div>
  <div class="compass-correction">
    <div>Compass Correction</div>
    <input
      type="number"
      bind:value={inputOffset}
      step="0.1"
      lang="en-US"
      placeholder="Enter Offset"
    />
    <!-- <button onclick={handleSetOffset} disabled={configStore.isLoading}>
      {configStore.isLoading ? "Saving..." : "Set"}
    </button>
    {#if configStore.error}
      <div class="error">Error: {configStore.error}</div>
    {/if} -->
    <button onclick={handleSetOffset}>Set</button>
  </div>
</div>

<style>
  .container {
    display: grid;
    grid-template-columns: 2fr 3fr;
  }
  .compass-display {
    display: flex;
    flex-direction: column;
    padding: auto;
  }
  .compass {
    position: relative;
    margin: 0 auto;
    display: flex;
    width: 80px;
    height: 80px;
    align-items: center;
    justify-content: center;
  }
  .rotating-circle {
    position: absolute;
    display: flex;
    width: 84px;
    height: 84px;
    border-radius: 50%;
    justify-content: center;
    margin: auto;
    color: yellow;
    font-size: 14pt;
  }
  .circle {
    position: absolute;
    width: 54px;
    height: 54px;
    border-radius: 50%;
    margin: auto;
    border: 1px solid yellow;
  }
  .arrow {
    width: 5px;
    height: 50%;
    background-color: white;
    margin: auto;
    border-radius: 4px;
  }
  .cmps-value {
    text-align: center;
    background-color: rgba(4, 61, 15, 0.5);
    color: white;
    font-size: 13pt;
    padding: 3px 6px;
    width: 60px;
    margin: 0 auto;
    margin-bottom: 6px;
    border-radius: 15px;
  }
  .compass-correction {
    display: flex;
    flex-direction: column;
    padding: 20px 6px 6px;
    align-items: center;
    gap: 8px;
    color: white;
  }
  .compass-correction > input {
    padding: 3px 8px;
    max-width: 100px;
  }
  .compass-correction > button {
    padding: 6px 16px;
  }
  .compass-correction > button:disabled {
    background-color: #ccc;
    cursor: not-allowed;
  }
</style>
