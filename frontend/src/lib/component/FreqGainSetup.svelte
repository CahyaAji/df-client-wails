<script lang="ts">
  import { setAntenna, setFreqGainApi } from "../utils/api_handler.js";
  import { signalState } from "../store/signalState.svelte.js";
  import { udpState } from "../store/udpStore.svelte.js";
  import DotNumberInput from "./DotNumberInput.svelte";

  let inputFreqMhz = $state(0);
  let errorMessage = $state("");
  let isBusy = $state(false);
  let errorTimeout: ReturnType<typeof setTimeout> | null = null;

  const currentStoreFreq = $derived(signalState.currentFreq);
  let isAutoMode = $derived(signalState.autoMode);
  const udpFreqHz = $derived(udpState.currentNumb);

  const displayFreqMhz = $derived(
    isAutoMode && udpFreqHz != null && udpFreqHz !== 0 && udpFreqHz > 0
      ? Number((udpFreqHz / 1_000_000).toFixed(3))
      : currentStoreFreq
  );

  $effect.pre(() => {
    if (!isAutoMode) {
      inputFreqMhz = currentStoreFreq;
    }
  });

  function showError(msg: string) {
    errorMessage = msg;
    if (errorTimeout) clearTimeout(errorTimeout);
    errorTimeout = setTimeout(() => {
      errorMessage = "";
    }, 1500);
  }

  async function handleSetAntenna(antSpace: number) {
    try {
      const result = await setAntenna(antSpace);
      if (!result.success) {
        console.log("API call failed:", result.error);
        return false;
      }
      console.log(`Antenna spacing set successfully: ${result.data}`);
      return true;
    } catch (error) {
      console.log("Error setting antenna:", error);
      return false;
    }
  }

  async function handleFreqSet() {
    if (isAutoMode || isBusy) return;
    if (inputFreqMhz < 24.0 || inputFreqMhz > 1000.0) {
      showError("Frequency must be between 24.0 and 1000.0 MHz");
      return;
    }

    signalState.setFrequency(inputFreqMhz);
    const antSpace = inputFreqMhz > 250 ? 0.25 : 0.45;
    isBusy = true;

    try {
      const antennaOK = await handleSetAntenna(antSpace);
      if (!antennaOK) showError("Error, Gagal mengatur antena");

      const result = await setFreqGainApi({
        center_freq: inputFreqMhz,
        uniform_gain: signalState.currentGain,
        ant_spacing_meters: antSpace,
      });

      if (!result.success) {
        showError("Error, Gagal mengatur frekuensi");
      } else {
        console.log("Frequency and gain set successfully:", result.data);
      }
    } catch (err) {
      console.error("Error in handleFreqSet:", err);
      showError("Error, Gagal mengatur frekuensi");
    } finally {
      isBusy = false;
    }
  }

  async function handleGainSet() {
    if (isBusy) return;
    isBusy = true;

    const freq = signalState.currentFreq;
    const antSpace = freq > 250 ? 0.25 : 0.45;

    try {
      const result = await setFreqGainApi({
        center_freq: freq,
        uniform_gain: signalState.currentGain,
        ant_spacing_meters: antSpace,
      });

      if (!result.success) {
        showError("Error, Gagal mengatur gain");
      } else {
        console.log("Gain set successfully:", result.data);
      }
    } catch (err) {
      console.error("Error in handleGainSet:", err);
      showError("Error, Gagal mengatur gain");
    } finally {
      isBusy = false;
    }
  }
</script>

<div class="container">
  <div class="content">
    <div class="input-field">
      <div class="label">Freq Mode</div>
      <div class="radio-group">
        <label class="radio-label">
          <input
            type="radio"
            name="frequency-mode"
            checked={isAutoMode === true}
            onchange={() => signalState.setAutoMode(true)}
          />
          <span>Auto</span>
        </label>
        <label class="radio-label">
          <input
            type="radio"
            name="frequency-mode"
            checked={isAutoMode === false}
            onchange={() => signalState.setAutoMode(false)}
          />
          <span>Manual</span>
        </label>
      </div>
    </div>
    <div class="input-field">
      <div class="label">Frequency</div>
      {#if isAutoMode}
        <DotNumberInput value={displayFreqMhz} disabled readonly />
      {:else}
        <DotNumberInput bind:value={inputFreqMhz} />
      {/if}
      <span>MHz</span>
      <button disabled={isAutoMode} onclick={handleFreqSet}>Set</button>
    </div>
    <div class="input-field">
      <div class="label">Gain</div>
      <select bind:value={signalState.currentGain}>
        <option value={0}>0.0</option>
        <option value={0.9}>0.9</option>
        <option value={1.4}>1.4</option>
        <option value={2.7}>2.7</option>
        <option value={3.7}>3.7</option>
        <option value={7.7}>7.7</option>
        <option value={8.7}>8.7</option>
        <option value={12.5}>12.5</option>
        <option value={14.4}>14.4</option>
        <option value={15.7}>15.7</option>
        <option value={16.6}>16.6</option>
        <option value={19.7}>19.7</option>
        <option value={20.7}>20.7</option>
        <option value={22.9}>22.9</option>
        <option value={25.4}>25.4</option>
        <option value={28.0}>28.0</option>
        <option value={29.7}>29.7</option>
        <option value={33.8}>33.8</option>
        <option value={36.4}>36.4</option>
        <option value={37.2}>37.2</option>
        <option value={38.6}>38.6</option>
        <option value={40.2}>40.2</option>
        <option value={42.1}>42.1</option>
        <option value={43.4}>43.4</option>
        <option value={44.5}>44.5</option>
        <option value={48.0}>48.0</option>
        <option value={49.6}>49.6</option>
      </select>
      <button onclick={handleGainSet}>Set</button>
    </div>
  </div>
</div>

<style>
  .container {
    color: white;
    display: flex;
    flex-direction: column;
  }
  .content {
    margin-top: 2px;
    margin-left: 8px;
    margin-right: 8px;
  }
  .input-field {
    display: flex;
    padding: 4px 0 8px 0;
  }
  .input-field > button {
    padding: 4px 18px;
    margin-left: 2px;
  }
  .input-field > span {
    margin: 0 4px;
    align-content: center;
  }
  .label {
    min-width: 80px;
    align-self: center;
    padding: 4px 0;
  }

  .radio-group {
    display: flex;
    gap: 10px;
  }
  .radio-label {
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
  }
  .radio-label input[type="radio"] {
    margin: 0;
  }

  .radio-label:hover {
    opacity: 0.8;
  }
</style>
