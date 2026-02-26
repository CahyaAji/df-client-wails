<script lang="ts">
    import { setStationId, turnOffDf, restartDf } from "../utils/api_handler";
    import { Quit } from "../../../wailsjs/runtime/runtime";
    import { signalState } from "../store/signalState.svelte";
    import { udpState, udpStore } from "../store/udpStore.svelte";
    import { dfStore } from "../store/dfStore.svelte";

    let message = $state("");
    let messageTimeout: ReturnType<typeof setTimeout> | null = null;
    let dfName = $state("");
    let isShuttingDown = $state(false);

    function showMessage(msg: string, duration: number = 1500) {
        message = msg;
        if (messageTimeout) clearTimeout(messageTimeout);
        messageTimeout = setTimeout(() => {
            message = "";
        }, duration);
    }

    async function closeApp() {
        if (isShuttingDown) return;
        isShuttingDown = true;
        console.log("Shutting down...");

        try {
            if (udpState.isListening) {
                udpStore.stopListening();
            }

            if (dfStore.isRunning) {
                dfStore.stop();
            }
        } catch (error) {
            console.error("Error during cleanup:", error);
        }

        setTimeout(() => {
            console.log("Exiting application.");
            Quit();
        }, 2500);
    }

    function handleTurnOff() {
        if (isShuttingDown) return;

        console.log("Turning off DF...");
        turnOffDf().catch((error) => console.error("TurnOff DF error:", error));

        showMessage(
            "Unit DF akan mati dalam ±20 detik, jangan langsung matikan daya DF",
            2300,
        );

        closeApp();
    }

    function handleRestart() {
        if (isShuttingDown) return;

        console.log("Restarting DF...");
        restartDf().catch((error) => console.error("Restart DF error:", error));

        showMessage(
            "Unit DF akan restart dalam ±60 detik, jangan langsung matikan daya DF",
            2300,
        );
        closeApp();
    }

    async function handleSetName() {
        if (!dfName) {
            showMessage("Error: Nama unit tidak boleh kosong");
            return;
        }

        try {
            const response = await setStationId(dfName);

            if (!response.success) {
                console.error("API call failed:", response.error);
                showMessage(
                    `Error: Gagal mengatur nama unit - ${response.error}`,
                );
                return;
            } else {
                console.log("Station name set successfully:", response.data);
                signalState.setStationName(dfName);
            }
        } catch (error) {
            console.error("error setStationName:", error);
            showMessage("Error: Gagal mengatur nama unit");
        }
    }

    $effect(() => {
        dfName = signalState.stationName;
    });
</script>

<div class="container">
    {#if message}
        <div class="msg">{message}</div>
    {:else}
        <div class="content">
            <label>
                <span>DF Unit Name :</span>
                <input
                    type="text"
                    bind:value={dfName}
                    disabled={isShuttingDown}
                />
                <button disabled={isShuttingDown} onclick={handleSetName}
                    >Set</button
                >
            </label>
            <div class="power-option">
                <div>Power Option :</div>
                <div class="option-btn">
                    <button disabled={isShuttingDown} onclick={handleRestart}
                        >Restart</button
                    >
                    <button disabled={isShuttingDown} onclick={handleTurnOff}
                        >Power OFF</button
                    >
                </div>
            </div>
        </div>
    {/if}
</div>

<style>
    button {
        padding: 4px 10px;
    }
    .content {
        display: flex;
        flex-direction: column;
        padding: 4px;
        color: white;
    }
    .content input {
        padding: 4px 8px;
        max-width: 120px;
    }
    .power-option {
        display: flex;
        margin-top: 10px;
        flex-direction: column;
        padding-right: 8px;
    }
    .option-btn {
        margin: 8px auto;
    }

    .msg {
        max-width: 220px;
        min-height: 60px;
        font-size: 16pt;
        margin: 20px auto;
        padding: 10px;
        border-radius: 8px;
        background-color: rgba(255, 0, 0, 0.8);
        color: white;
        text-align: center;
    }
</style>
