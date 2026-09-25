<script lang="ts">
    import { dfStore } from "../store/dfStore.svelte.js";

    // Throttled copy of the heading so the arrow only re-renders at most
    // once per second, regardless of how many times dfStore mutates its
    // $state fields within a single poll cycle.
    let displayHeading = $state<number | null>(null);
    let _throttleTimer: ReturnType<typeof setTimeout> | null = null;
    const THROTTLE_MS = 1000;

    $effect(() => {
        const heading = dfStore.data?.heading;
        const hasHeading = heading !== undefined && heading !== null;

        if (!hasHeading) {
            // Data went away — clear immediately so "---" shows without delay.
            displayHeading = null;
            if (_throttleTimer) {
                clearTimeout(_throttleTimer);
                _throttleTimer = null;
            }
            return;
        }

        if (_throttleTimer) {
            // A throttle window is already open; skip this update.
            // The next poll cycle will pick up the latest value after
            // the window closes.
            return;
        }

        displayHeading = heading;
        _throttleTimer = setTimeout(() => {
            _throttleTimer = null;
        }, THROTTLE_MS);

        return () => {
            // Cleanup when effect re-runs or component is destroyed.
            if (_throttleTimer) {
                clearTimeout(_throttleTimer);
                _throttleTimer = null;
            }
        };
    });
</script>


<div class="container">
    {#if displayHeading !== null}
        <div
            class="rotating-circle"
            style="transform: rotate({displayHeading}deg);"
        >
            <div class="arrow"></div>
        </div>
    {/if}
    <div class="angle-text">
        {#if displayHeading !== null}
            <div>{displayHeading}</div>
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
