<script lang="ts">
    import { dfStore } from "../store/dfStore.svelte.js";
    import { configStore } from "../store/configStore.svelte.js";
    import { signalState } from "../store/signalState.svelte.js";

    console.log("RelativePlot, dfStore data:", dfStore.data);
    console.log("df heading:", dfStore.data?.heading);
</script>

<div class="container">
    {#if dfStore.data && dfStore.data.heading !== undefined && dfStore.data.heading !== null}
        <div
            class="rotating-circle"
            style="transform: rotate({dfStore.data.heading}deg);"
        >
            <div class="arrow"></div>
        </div>
    {/if}
    <div class="angle-text">
        {#if dfStore.data && dfStore.data.heading !== undefined && dfStore.data.heading !== null}
            <div>{dfStore.data.heading}</div>
        {:else}
            <div>---</div>
        {/if}
    </div>
</div>

<style>
    .container {
        display: flex;
        margin: auto;
        width: 210px;
        height: 210px;
        background-image: url("/src/assets/relative_circle.png");
        background-repeat: no-repeat;
        background-size: cover;
        background-color: rgba(4, 61, 15, 0.5);
        border-radius: 50%;
        position: relative;
        align-items: center;
        justify-content: center;
    }
    .rotating-circle {
        background-color: transparent;
        width: 170px;
        height: 170px;
        border-radius: 50%;
    }
    .arrow {
        width: 8px;
        height: 56px;
        background-color: yellow;
        margin: auto;
        border-radius: 4px;
    }
    .angle-text {
        width: 64px;
        height: 64px;
        display: flex;
        position: absolute;
        align-items: center;
        justify-content: center;
        border-radius: 50%;
        border: 2px solid white;
        color: white;
        background-color: rgba(4, 61, 15, 0.5);
        color: yellow;
        font-size: 18pt;
    }
</style>
