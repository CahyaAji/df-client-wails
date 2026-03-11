import { readDF } from "../utils/api_handler.js";
// import { configStore } from "./configStore.svelte.js";
// import { signalState } from "../store/signalState.svelte.js";


/**
 * @typedef {Object} DFData
 * @property {string} time
 * @property {number} heading
 * @property {string} confidence
 * @property {string} power
 * @property {number[]} polar
 */

class DFStore {
    data = $state(/** @type {DFData | null} */ (null));
    error = $state(/** @type {string | null} */ (null));
    isLoading = $state(false);
    lastTimestamp = $state(/** @type {string | null} */ (null));

    /** @type {ReturnType<typeof setTimeout> | null} */
    #timer = null;
    #running = false;

    get isRunning() {
        return this.#running;
    }

    async fetch() {
        this.isLoading = true;
        this.error = null;

        try {
            const result = await readDF();

            if (result.success && result.data) {
                const newTimestamp = result.data.time;

                if (
                    this.lastTimestamp === null ||
                    newTimestamp !== this.lastTimestamp
                ) {
                    this.data = result.data;
                    this.lastTimestamp = newTimestamp;
                    console.log("New DF data received, timestamp:", newTimestamp);
                    // if (signalState.currentFreq > 250) {
                    //     console.log("offsetUhf applied:", configStore.offsetUhf);
                    //     this.data.heading += configStore.offsetUhf;
                    // } else {
                    //     console.log("offsetVhf applied:", configStore.offsetVhf);
                    //     this.data.heading += configStore.offsetVhf;
                    // }
                } else {
                    console.log("Stale DF data detected, same timestamp:", newTimestamp);

                    this.data = null;
                    this.error = "No new data available";
                }
            } else {
                this.data = null;
                this.error = result.success ? "Invalid response: missing data" : (result.error ?? "Unknown error");
                // Don't clear lastTimestamp on API errors - might be temporary network issue
            }

            this.isLoading = false;
            return result;
        } catch (error) {
            this.data = null;
            this.error = error instanceof Error ? error.message : String(error);
            this.isLoading = false;
            // Don't clear lastTimestamp on network errors
            return { success: false, error: this.error };
        }
    }

    async #poll() {
        if (!this.#running) return;
        await this.fetch();
        if (this.#running) {
            this.#timer = setTimeout(() => this.#poll(), 1000);
        }
    }

    start() {
        if (this.#running) return;
        this.#running = true;
        // Defer the very first request so the UI renders before any network
        // activity begins.
        this.#timer = setTimeout(() => this.#poll(), 100);
        console.log("DF Store started");
    }

    stop() {
        this.#running = false;
        if (this.#timer) {
            clearTimeout(this.#timer);
            this.#timer = null;
        }
        console.log("DF monitoring stopped");
    }

}

export const dfStore = new DFStore();
