<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import {
        API_URL,
        getDFSettings,
        setAntenna,
        setFreqGainApi,
    } from "../utils/api_handler.js";
    import RelativePlot from "./RelativePlot.svelte";
    import ControlPanel from "./ControlPanel.svelte";
    import { dfStore } from "../store/dfStore.svelte.js";
    import { configStore } from "../store/configStore.svelte.js";
    import { compassStore } from "../store/compassStore.svelte.js";
    import { signalState } from "../store/signalState.svelte.js";
    import { udpState, udpStore } from "../store/udpStore.svelte.js";

    let isStatusOpen = $state(true);
    let isPlotOpen = $state(true);
    let isSettingsOpen = $state(true);

    let frequencyDebounceTimer: ReturnType<typeof setTimeout> | null = null;
    const FREQUENCY_DEBOUNCE_MS = 150;
    const MIN_FREQUENCY_CHANGE = 0.001;

    let prevFreq = $state(0);
    let isChangingFreq = $state(false);
    let retryCount = $state(0);
    const MAX_RETRIES = 3;

    /**
     * @param {number} antSpace
     */
    async function handleSetAntenna(antSpace: number) {
        try {
            const result = await setAntenna(antSpace);
            if (result.success) {
                return true;
            } else {
                console.log(`Antenna setting failed: ${result.error}`);
                return false;
            }
        } catch (error) {
            console.log("Error setting antenna:", error);
            return false;
        }
    }

    /**
     * @param {number} newFreq
     * @param {number} newGain
     * @param {number} antSpace
     */
    async function handleSetFreqAndGain(
        newFreq: number,
        newGain: number,
        antSpace: number,
    ) {
        try {
            const apiData = {
                center_freq: newFreq,
                uniform_gain: newGain,
                ant_spacing_meters: antSpace,
            };

            const result = await setFreqGainApi(apiData);
            if (result.success) {
                signalState.setFrequency(newFreq);
                signalState.setGain(newGain);
                return true;
            } else {
                console.error(
                    "Failed to set frequency and gain:",
                    result.error,
                );
                return false;
            }
        } catch (error) {
            console.error("Error setting frequency and gain:", error);
            return false;
        }
    }

    /**
     * @param {number} [newFreq]
     * @param {number} [newGain]
     */
    async function handleSetFreq(newFreq: number, newGain: number) {
        if (isChangingFreq) {
            console.log("Frequency change already in progress, skipping");
            return;
        }

        if (newFreq === prevFreq && retryCount === 0) {
            console.log("Frequency unchanged, skipping");
            return;
        }

        isChangingFreq = true;
        console.log(
            `handleSetFreq called, freq: ${newFreq}, retry: ${retryCount}`,
        );

        const antSpace = newFreq >= 250 ? 0.25 : 0.45;
        let antennaSuccess = false;
        let frequencySuccess = false;

        try {
            // STEP 1: Always set antenna first
            console.log(
                `Setting antenna spacing to ${antSpace}m for frequency ${newFreq}MHz`,
            );
            antennaSuccess = await handleSetAntenna(antSpace);

            if (!antennaSuccess) {
                console.log(
                    "Antenna setting failed, but continuing with frequency setting",
                );
            }

            // STEP 2: Set frequency and gain
            frequencySuccess = await handleSetFreqAndGain(
                newFreq,
                newGain,
                antSpace,
            );

            if (frequencySuccess) {
                prevFreq = newFreq;
                retryCount = 0;
                console.log(
                    `All settings applied successfully - Antenna: ${antennaSuccess ? "OK" : "FAILED"}, Frequency: OK`,
                );
            } else {
                throw new Error("Frequency setting failed");
            }
        } catch (error) {
            console.error("Error in frequency setting process:", error);

            // Retry logic - retry the entire process (antenna + frequency)
            if (retryCount < MAX_RETRIES) {
                retryCount++;
                console.log(
                    `Retrying entire process (attempt ${retryCount}/${MAX_RETRIES})`,
                );

                setTimeout(
                    () => {
                        isChangingFreq = false;
                        handleSetFreq(newFreq, newGain);
                    },
                    1000 + retryCount * 500,
                );
                return;
            } else {
                console.error(`Failed after ${MAX_RETRIES} attempts`);
                retryCount = 0;
            }
        } finally {
            isChangingFreq = false;
            console.log("handleSetFreq finished");
        }
    }

    // UDP management
    $effect(() => {
        if (signalState.autoMode) {
            if (!udpState.isListening) {
                udpStore
                    .startListening(49876)
                    .then((result) => console.log("UDP started:", result))
                    .catch((error) => console.log("UDP error:", error.message));
            }
        } else {
            if (udpState.isListening) {
                udpStore
                    .stopListening()
                    .then((result) => console.log("UDP stopped:", result))
                    .catch((error) =>
                        console.log("UDP stop error:", error.message),
                    );
            }
        }
    });

    $effect(() => {
        if (!signalState.autoMode) {
            // Clear any pending debounced calls
            if (frequencyDebounceTimer) {
                clearTimeout(frequencyDebounceTimer);
                frequencyDebounceTimer = null;
            }
            return;
        }

        if (
            udpState.currentNumb === null ||
            udpState.currentNumb < 24000000 ||
            udpState.currentNumb > 1000000000 ||
            !udpState.isListening
        ) {
            return;
        }

        const freqInMhz = Number((udpState.currentNumb / 1000000).toFixed(3));

        if (!Number.isFinite(freqInMhz) || Number.isNaN(freqInMhz)) {
            console.error(
                "Frequency conversion resulted in invalid number:",
                freqInMhz,
            );
            return;
        }

        console.log(
            "Processing frequency:",
            freqInMhz,
            "prevFreq:",
            prevFreq,
            "difference:",
            Math.abs(freqInMhz - prevFreq),
        );

        const frequencyDifference = Math.abs(freqInMhz - prevFreq);
        if (frequencyDifference < MIN_FREQUENCY_CHANGE) {
            console.log("Frequency change too small, ignoring");
            return;
        }

        if (frequencyDebounceTimer) {
            clearTimeout(frequencyDebounceTimer);
        }

        frequencyDebounceTimer = setTimeout(() => {
            console.log("Debounced frequency change triggered:", freqInMhz);

            if (
                signalState.autoMode &&
                udpState.isListening &&
                !isChangingFreq &&
                Math.abs(freqInMhz - prevFreq) >= MIN_FREQUENCY_CHANGE
            ) {
                const currentGain = signalState.currentGain;
                console.log(
                    "Calling handleSetFreq with:",
                    freqInMhz,
                    "gain:",
                    currentGain,
                );
                handleSetFreq(freqInMhz, currentGain);
            } else {
                console.log(
                    "Conditions changed during debounce, skipping frequency update",
                );
            }

            frequencyDebounceTimer = null;
        }, FREQUENCY_DEBOUNCE_MS);

        return () => {
            if (frequencyDebounceTimer) {
                clearTimeout(frequencyDebounceTimer);
                frequencyDebounceTimer = null;
            }
        };
    });

    onMount(() => {
        // 1. Load config from Go backend (fast local IPC, never blocks)
        configStore.load();

        // 2. Start serialized polling stores
        if (!dfStore.isRunning) dfStore.start();
        if (!compassStore.isRunning) compassStore.start();

        // 3. Load initial device settings in the background — never blocks startup
        (async () => {
            try {
                const savedDFSettings = await getDFSettings();
                const centerFreq = Number(savedDFSettings.center_freq || 0);
                signalState.setFrequency(centerFreq);
                signalState.setGain(Number(savedDFSettings.uniform_gain || 0));
                signalState.setStationName(savedDFSettings.station_id);
                if (centerFreq > 0) {
                    const antSpace = centerFreq >= 250 ? 0.25 : 0.45;
                    await handleSetAntenna(antSpace);
                }
            } catch (error) {
                console.log("Failed to load initial settings:", error);
            }
        })();
    });

    onDestroy(async () => {
        if (frequencyDebounceTimer) {
            clearTimeout(frequencyDebounceTimer);
            frequencyDebounceTimer = null;
        }
        dfStore.stop();
        compassStore.stop();
        try {
            await udpStore.stopListening();
        } catch (err) {
            console.log("UDP stop error:", (err as Error).message);
        }
    });
</script>

<div class="container">
    <div style="width: 100%;">
        <div class="title">
            <button
                style="margin-left: 4px;"
                onclick={() => {
                    console.log("DFPanel status toggle", { isStatusOpen });
                    isStatusOpen = !isStatusOpen;
                }}
            >
                {isStatusOpen ? "︽" : "︾"}
            </button>
            <div style="margin: 6px 4px;">Status</div>
        </div>
        {#if isStatusOpen}
            <iframe
                class="webview"
                src={`${API_URL}/config`}
                title="Status"
                style="width: 290px; margin: auto; padding-left: 13px; height: 184px; border: none; overflow-x: hidden; background-color: rgba(4, 61, 15, 0.2);"
            ></iframe>
        {/if}
    </div>
    <div style="width: 100%;">
        <div class="title">
            <button
                style="margin-left: 4px;"
                onclick={() => {
                    console.log("DFPanel plot toggle", { isPlotOpen });
                    isPlotOpen = !isPlotOpen;
                }}
            >
                {isPlotOpen ? "︽" : "︾"}
            </button>
            <div style="margin: 6px 4px;">Plot</div>
        </div>
        {#if isPlotOpen}
            <RelativePlot />
        {/if}
    </div>
    <div style="width: 100%;">
        <div class="title">
            <button
                style="margin-left: 4px;"
                onclick={() => {
                    console.log("DFPanel settings toggle", { isSettingsOpen });
                    isSettingsOpen = !isSettingsOpen;
                }}
            >
                {isSettingsOpen ? "︽" : "︾"}
            </button>
            <div style="margin: 6px 4px;">Settings</div>
        </div>
        {#if isSettingsOpen}
            <ControlPanel />
        {/if}
    </div>
</div>

<style>
    .container {
        border-radius: 10px;
        position: fixed;
        top: 8px;
        right: 8px;
        display: flex;
        flex-direction: column;
        gap: 1px;
        box-shadow: 0 3px 8px rgba(0, 0, 0, 0.2);
        border: 1px solid rgba(0, 0, 0, 0.1);
        overflow: hidden;
        align-items: stretch;
        background-color: transparent;
        touch-action: manipulation;
        z-index: 12000;
        pointer-events: auto;
        -webkit-app-region: no-drag;
    }
    .container > div {
        touch-action: manipulation;
        font-size: 14px;
        font-weight: 500;
        color: #1e293b;
        border-bottom: 1px solid rgba(0, 0, 0, 0.1);
        user-select: none;
    }
    .title {
        border-bottom: 1px solid black;
        display: flex;
        align-items: center;
        gap: 4px;
        background-color: rgba(4, 61, 15, 0.6);
        color: white;
        font-size: 13pt;
        pointer-events: auto;
        -webkit-app-region: no-drag;
    }
    .title button {
        pointer-events: auto;
        touch-action: manipulation;
        cursor: pointer;
        -webkit-app-region: no-drag;
    }
</style>
