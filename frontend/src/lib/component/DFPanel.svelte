<script lang="ts">
    import { API_URL } from "../utils/api_handler.js";
    import RelativePlot from "./RelativePlot.svelte";
    import ControlPanel from "./ControlPanel.svelte";

    let isStatusOpen = $state(true);
    let isPlotOpen = $state(true);
    let isSettingsOpen = $state(true);
</script>

<div class="container">
    <div style="width: 100%;">
        <div class="title">
            <div style="margin: 6px 4px;">Status</div>
            <button onclick={() => isStatusOpen = !isStatusOpen}>
                    {isStatusOpen ? "︽" : "︾"}
            </button>
        </div>
        {#if isStatusOpen}
        <iframe
            class="webview"
            src={`${API_URL}/config`}
            title="Example website"
            style="width: 290px; margin: auto; height: 100%; border: none; overflow-x: hidden; background-color: rgba(4, 61, 15, 0.2);"
        ></iframe>
        {/if}
    </div>
    <div style="width: 100%;">
        <div class="title">
             <div style="margin: 6px 4px;">Plot</div>
            <button onclick={() => isPlotOpen = !isPlotOpen}>
                {isPlotOpen ? "︽" : "︾"}
            </button>
        </div>
        {#if isPlotOpen}
        <RelativePlot />
        {/if}
    </div>
    <div style="width: 100%;">
        <div class="title">
            <div style="margin: 6px 4px;">Settings</div>
            <button onclick={() => isSettingsOpen = !isSettingsOpen}>
                {isSettingsOpen ? "︽" : "︾"}
            </button>
        </div>
        {#if isSettingsOpen}
        <ControlPanel />
        {/if}
    </div>
    
</div>

<style>
    .container {
        border-radius: 10px;
        position: absolute;
        display: flex;
        flex-direction: column;
        top: 8px;
        right: 8px;
        gap: 1px;
        box-shadow: 0 3px 8px rgba(0, 0, 0, 0.2);
        border: 1px solid rgba(0, 0, 0, 0.1);
        overflow: hidden;
        align-items: center;
        background-color: transparent;
    }
    .title {
        border-bottom: 1px solid black;
        display: flex;
        align-items: center;
        gap: 4px;
        background-color: rgba(4, 61, 15, 0.6);
        color: white;
        font-size: 13pt;
    }
    .container > div {
        font-size: 14px;
        font-weight: 500;
        color: #1e293b;
        border-bottom: 1px solid rgba(0, 0, 0, 0.1);
    }
</style>
