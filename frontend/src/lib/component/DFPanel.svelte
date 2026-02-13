<script lang="ts">
    import { API_URL } from "../utils/api_handler.js";
    import RelativePlot from "./RelativePlot.svelte";
    import ControlPanel from "./ControlPanel.svelte";
</script>

<div class="container">
    <div>
        <div class="title">
            <div style="margin: 6px 4px;">Status</div>
            <button>︽︾</button>
        </div>
        <iframe
            class="webview"
            src={`${API_URL}/config`}
            title="Example website"
            style="width: 100%; height: 100%; border: none; overflow-x: hidden; background-color: rgba(4, 61, 15, 0.2);"
        ></iframe>
    </div>
    <div>
        <div class="title">
            <div>Plot</div>
            <button>︽︾</button>
        </div>
        <RelativePlot />
    </div>
    <ControlPanel />
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
