class SignalState {
  currentFreq = $state(0);
  currentGain = $state(0);
  autoMode = $state(false);
  stationName = $state("");

  /**
   * @param {number} freq
   */
  setFrequency(freq) {
    this.currentFreq = freq;
  }

  /**
   * @param {number} gain
   */
  setGain(gain) {
    this.currentGain = gain;
  }

  /**
   * @param {boolean} auto
   */
  setAutoMode(auto) {
    this.autoMode = auto;
  }

  /**
   * @param {string} name
   */
  setStationName(name) {
    this.stationName = name;
  }

  reset() {
    this.currentFreq = 0;
    this.currentGain = 0;
    this.autoMode = true;
    this.stationName = "";
  }
}

export const signalState = new SignalState();
