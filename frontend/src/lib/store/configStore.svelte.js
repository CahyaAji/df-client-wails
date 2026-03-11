import {
  GetConfig,
  SetOffsetUhf,
  SetOffsetVhf,
  SetCompassOffset,
  SetGPSLocation,
  SetUTMLocation,
  ResetConfig,
} from "../../../wailsjs/go/main/App";

class ConfigStore {
  mapKey = $state("");
  compassOffset = $state(0);
  offsetUhf = $state(0);
  offsetVhf = $state(0);
  gpsLocation = $state({ lat: 0, lng: 0 });
  utmLocation = $state({ zone: "", easting: "", northing: "", co: "" });

  #isLoading = $state(false);
  /** @type {string | null} */
  #error = $state(null);

  get isLoading() {
    return this.#isLoading;
  }

  get error() {
    return this.#error;
  }

  get allSettings() {
    return {
      mapKey: this.mapKey,
      compassOffset: this.compassOffset,
      offsetUhf: this.offsetUhf,
      offsetVhf: this.offsetVhf,
      gpsLocation: this.gpsLocation,
      utmLocation: this.utmLocation,
    };
  }

  async load() {
    this.#isLoading = true;
    this.#error = null;
    try {
      const config = await GetConfig();
      this.mapKey = config.map_key ?? "";
      this.compassOffset = config.compass_offset ?? 0;
      this.offsetUhf = config.offsetUhf ?? 0;
      this.offsetVhf = config.offsetVhf ?? 0;
      this.gpsLocation = config.gps_location ?? { lat: 0, lng: 0 };
      this.utmLocation = config.utm_location ?? {
        zone: "",
        easting: "",
        northing: "",
        co: "",
      };
      console.log("Settings loaded successfully:", this.allSettings);
      this.#isLoading = false;
      return { success: true, data: this.allSettings };
    } catch (error) {
      console.error("Failed to load settings:", error);
      this.#error = error instanceof Error ? error.message : String(error);
      this.#isLoading = false;
      return { success: false, error: this.#error };
    }
  }

  /**
   * @param {number} value
   */
  async setOffsetUhf(value) {
    const oldValue = this.offsetUhf;
    this.offsetUhf = Number(value);
    try {
      await SetOffsetUhf(this.offsetUhf);
      console.log("UHF offset saved:", this.offsetUhf);
      return { success: true };
    } catch (error) {
      this.offsetUhf = oldValue;
      console.error("Failed to save UHF offset, reverting to:", oldValue);
      this.#error = error instanceof Error ? error.message : String(error);
      return { success: false, error: this.#error };
    }
  }

  /**
   * @param {number} value
   */
  async setOffsetVhf(value) {
    const oldValue = this.offsetVhf;
    this.offsetVhf = Number(value);
    try {
      await SetOffsetVhf(this.offsetVhf);
      console.log("VHF offset saved:", this.offsetVhf);
      return { success: true };
    } catch (error) {
      this.offsetVhf = oldValue;
      console.error("Failed to save VHF offset, reverting to:", oldValue);
      this.#error = error instanceof Error ? error.message : String(error);
      return { success: false, error: this.#error };
    }
  }

  /**
   * @param {number} value
   */
  async setCompassOffset(value) {
    const oldValue = this.compassOffset;
    this.compassOffset = Number(value);
    try {
      await SetCompassOffset(this.compassOffset);
      console.log("Compass offset saved:", this.compassOffset);
      return { success: true };
    } catch (error) {
      this.compassOffset = oldValue;
      console.error("Failed to save compass offset, reverting to:", oldValue);
      this.#error = error instanceof Error ? error.message : String(error);
      return { success: false, error: this.#error };
    }
  }

  /**
   * @param {number} lat
   * @param {number} lng
   */
  async setGPSLocation(lat, lng) {
    const oldValue = { ...this.gpsLocation };
    this.gpsLocation = { lat: Number(lat), lng: Number(lng) };
    try {
      await SetGPSLocation(Number(lat), Number(lng));
      console.log("GPS location saved:", $state.snapshot(this.gpsLocation));
      return { success: true };
    } catch (error) {
      this.gpsLocation = oldValue;
      console.error("Failed to save GPS location, reverting to:", $state.snapshot(oldValue));
      this.#error = error instanceof Error ? error.message : String(error);
      return { success: false, error: this.#error };
    }
  }

  /**
   * @param {string} zone
   * @param {string} easting
   * @param {string} northing
   * @param {string} co
   */
  async setUTMLocation(zone, easting, northing, co) {
    const oldValue = { ...this.utmLocation };
    this.utmLocation = {
      zone: String(zone),
      easting: String(easting),
      northing: String(northing),
      co: String(co),
    };
    try {
      await SetUTMLocation(String(zone), String(easting), String(northing), String(co));
      console.log("UTM location saved:", $state.snapshot(this.utmLocation));
      return { success: true };
    } catch (error) {
      this.utmLocation = oldValue;
      console.error("Failed to save UTM location, reverting to:", $state.snapshot(oldValue));
      this.#error = error instanceof Error ? error.message : String(error);
      return { success: false, error: this.#error };
    }
  }

  async reset() {
    this.compassOffset = 0;
    this.offsetUhf = 0;
    this.offsetVhf = 0;
    this.gpsLocation = { lat: 0, lng: 0 };
    this.utmLocation = { zone: "", easting: "", northing: "", co: "" };
    try {
      await ResetConfig();
      console.log("Settings reset to defaults");
      return { success: true };
    } catch (error) {
      console.error("Failed to reset settings:", error);
      this.#error = error instanceof Error ? error.message : String(error);
      return { success: false, error: this.#error };
    }
  }
}

export const configStore = new ConfigStore();
