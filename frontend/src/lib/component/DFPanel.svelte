<script lang="ts">
    import { API_URL } from "../utils/api_handler.js";
    import RelativePlot from "./RelativePlot.svelte";
    import ControlPanel from "./ControlPanel.svelte";
    import { dfStore } from "../store/dfStore.svelte.js";
    import { configStore } from "../store/configStore.svelte.js";
    import { compassStore } from "../store/compassStore.svelte.js";
    import { signalState } from "../store/signalState.svelte.js";
    import { getDFSettings } from "../utils/api_handler.js";
    import { udpState, udpStore } from "../store/udpStore.svelte.js";
    import { setAntenna, setFreqGainApi } from "../utils/api_handler.js";

    let isStatusOpen = $state(true);
    let isPlotOpen = $state(true);
    let isSettingsOpen = $state(true);

    let appInitialized = false;
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

    $effect(() => {
        console.log("DFPanel, appInitialized:", appInitialized);

        if (appInitialized) {
            console.log("DFPanel already initialized, skipping");
            return () => {
                console.log("Skipped initialization cleanup - doing nothing");
            };
        }

        async function initialize() {
            appInitialized = true;

            // 1. Load ConfigStore
            try {
                const configResult = await configStore.load();
                if (configResult.success) {
                    console.log(
                        "configStore loaded succesfully:",
                        configStore.allSettings,
                    );
                } else {
                    console.log(
                        "Config load failed, using defaults:",
                        configResult.error,
                    );
                }
            } catch (error) {
                console.log("error loading configStore:", error);
            }
            // 2. Start DFStore
            if (!dfStore.isRunning) {
                console.log("Starting dfStore");
                dfStore.start();
                console.log("dfStore started");
            } else {
                console.log("dfStore already running");
            }
            //3. Start CompassStore
            if (!compassStore.isRunning) {
                try {
                    console.log("Starting compassStore");
                    compassStore.start();
                    console.log("compassStore started");
                } catch (error) {
                    console.error("Failed to start compassStore:", error);
                }
            }
            //4 Load DF Setting
            try {
                const savedDFSettings = await getDFSettings();
                const centerFreq = Number(savedDFSettings.center_freq || 0);
                signalState.setFrequency(centerFreq);

                signalState.setGain(Number(savedDFSettings.uniform_gain || 0));
                signalState.setStationName(savedDFSettings.station_id);

                // setAntenna for initial freq
                if (centerFreq > 0) {
                    const antSpace = centerFreq >= 250 ? 0.25 : 0.45;
                    await handleSetAntenna(antSpace);
                }

                console.log("Initial settings loaded:", savedDFSettings);
            } catch (error) {
                console.log("Failed to load initial setting config:", error);
            }
            console.log("App initialization completed");
        }

        initialize();

        return async () => {
            //freq debounce
            if (frequencyDebounceTimer) {
                clearTimeout(frequencyDebounceTimer);
                frequencyDebounceTimer = null;
            }

            // stop df store
            dfStore.stop();

            // stop compass listening
            compassStore.stop();

            //stop udp listening
            try {
                const result = await udpStore.stopListening();
                console.log("Parent destroy:", result);
            } catch (err) {
                console.log("UDP stop error:", (err as Error).message);
            }
        };
    });
</script>

<div class="container">
    <div style="width: 100%;">
        <div class="title">
            <div style="margin: 6px 4px;">Status</div>
            <button onclick={() => (isStatusOpen = !isStatusOpen)}>
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
            <button onclick={() => (isPlotOpen = !isPlotOpen)}>
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
            <button onclick={() => (isSettingsOpen = !isSettingsOpen)}>
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
