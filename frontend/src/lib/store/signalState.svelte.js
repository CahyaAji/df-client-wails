class SignalState {
  currentFreq = $state(0);
  currentGain = $state(0);
  compassOffset = $state(0);
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

  /**
   * @param {number} offset
   */
  setCompassOffset(offset) {
    this.compassOffset = offset;
  }

  reset() {
    this.currentFreq = 0;
    this.currentGain = 0;
    this.compassOffset = 0;
    this.autoMode = false;
    this.stationName = "";
  }
}

export const signalState = new SignalState();
