<script lang="ts">
  import CompassSetup from "./CompassSetup.svelte";
  import FreqGainSetup from "./FreqGainSetup.svelte";
  import GPSSetup from "./GPSSetup.svelte";
  import Option from "./Option.svelte";

  let activeTab = "frequency";

  /**
   * @param {string} tabName
   */
  function switchToTab(tabName: string) {
    activeTab = tabName;
  }
</script>

<div class="container">
  <div class="navigation">
    <button
      class:active={activeTab === "frequency"}
      onclick={() => switchToTab("frequency")}>Frequency</button
    >
    <button
      class:active={activeTab === "compass"}
      onclick={() => switchToTab("compass")}>Compass</button
    >
    <button
      class:active={activeTab === "gps"}
      onclick={() => switchToTab("gps")}>GPS</button
    >
    <button
      class:active={activeTab === "option"}
      onclick={() => switchToTab("option")}>Option</button
    >
  </div>
  {#if activeTab === "frequency"}
    <FreqGainSetup />
  {:else if activeTab === "compass"}
    <CompassSetup />
  {:else if activeTab === "gps"}
    <GPSSetup />
  {:else if activeTab === "option"}
    <Option />
  {/if}
</div>

<style>
  .container {
    margin-top: 4px;
    display: flex;
    flex-direction: column;
    background-color: rgba(75, 83, 32, 0.6);
    overflow: auto;
  }
  .navigation {
    display: flex;
    padding: 0;
    flex-shrink: 0;
    gap: 0;
    border-bottom: 1px solid white;
    background-color: rgba(4, 61, 15, 0.5);
  }
  .navigation > button {
    padding: 4px 6px;
    margin: 0;
    cursor: pointer;
    transition: all 0.2s ease;
    color: white;
    border: 0;
    background-color: transparent;
    font-size: 13pt;
    opacity: 0.8;
  }
  .navigation > button.active {
    opacity: 1;
    background-color: rgba(4, 61, 15, 0.8);
  }
</style>
