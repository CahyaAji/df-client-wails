<script lang="ts">
    import { dfStore } from "../store/dfStore.svelte.js";

    // Throttled copy of the heading (leading + trailing edge): the first value
    // shows immediately, and the latest value seen during the window is
    // applied when it closes, so the arrow never stays stale.
    let displayHeading = $state<number | null>(null);
    const THROTTLE_MS = 1000;
    let throttleTimer: ReturnType<typeof setTimeout> | null = null;
    let pending: number | null = null;

    function openWindow() {
        throttleTimer = setTimeout(() => {
            throttleTimer = null;
            if (pending !== null) {
                displayHeading = pending;
                pending = null;
                openWindow();
            }
        }, THROTTLE_MS);
    }

    $effect(() => {
        const heading = dfStore.data?.heading;

        if (heading === undefined || heading === null) {
            pending = null;
            displayHeading = null;
            if (throttleTimer) {
                clearTimeout(throttleTimer);
                throttleTimer = null;
            }
            return;
        }

        if (throttleTimer) {
            pending = heading;
            return;
        }

        displayHeading = heading;
        openWindow();
    });

    // Only clear the timer on destroy. Doing it in the effect's cleanup would
    // cancel the throttle on every heading change.
    $effect(() => () => {
        if (throttleTimer) clearTimeout(throttleTimer);
        throttleTimer = null;
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
