
class LocationStore {
  /** @type {{ lat: number, lon: number, acc: number, heading: number, timestamp: number, speed?: number } | null} */
  data = $state(null);
  /** @type {string | null} */
  error = $state(null);
  isLoading = $state(false);
  hasHardware = $state(true);
  /** @type {number | null} */
  #watchId = null;

  get isRunning() {
    return this.#watchId !== null;
  }

  start() {
    if (!("geolocation" in navigator)) {
      this.error = "Geolocation API not supported";
      this.hasHardware = false;
      console.warn("❌ Geolocation API not available, using IP fallback");
      this.fallbackIP();
      return;
    }

    if (this.#watchId !== null) {
      //already running
      return;
    }

    this.isLoading = true;
    this.error = null;
    this.hasHardware = true;

    this.#watchId = navigator.geolocation.watchPosition(
      (pos) => {
        const acc = pos.coords.accuracy;

        this.data = {
          lat: pos.coords.latitude,
          lon: pos.coords.longitude,
          acc,
          heading: pos.coords.heading ?? 0,
          timestamp: pos.timestamp,
        };

        this.isLoading = false;

        if (acc > 100) this.hasHardware = false;
      },
      (err) => {
        console.error("📡 Geolocation error:", err.code, err.message);
        this.error = err.message;
        this.isLoading = false;
        this.hasHardware = false;

        // Fallback to IP-based location on any geolocation error
        // This is especially important for Linux/macOS where permissions may not work
        if (
          err.code === err.PERMISSION_DENIED ||
          err.code === err.POSITION_UNAVAILABLE
        ) {
          console.warn("⚠️ GPS unavailable (code: " + err.code + "), falling back to IP location");
          this.fallbackIP();
        }
      },
      {
        enableHighAccuracy: true,
        maximumAge: 0,
        timeout: 10000,
      }
    );
    console.log("📡 Location store started");
  }

  stop() {
    if (this.#watchId !== null) {
      navigator.geolocation.clearWatch(this.#watchId);
      this.#watchId = null;
      console.log("📡 Location store stopped");
    }
  }

  clear() {
    this.data = null;
    this.error = null;
    this.isLoading = false;
    this.hasHardware = true;
  }

  async fallbackIP() {
    try {
      console.log("🌐 Fetching IP-based location...");
      const res = await fetch("https://ipapi.co/json/");
      const j = await res.json();

      this.data = {
        lat: j.latitude,
        lon: j.longitude,
        acc: 5000, // IP location is very inaccurate
        speed: 0,
        heading: 0,
        timestamp: Date.now(),
      };

      this.error = null;
      this.isLoading = false;
      this.hasHardware = false;
      console.log("✅ Using IP fallback location:", j.city, j.country_name);
    } catch (e) {
      console.error("❌ IP fallback failed:", e);
      const message = e instanceof Error ? e.message : String(e);
      this.error = "Failed to get location: " + message;
      this.isLoading = false;
    }
  }
}

export const locationStore = new LocationStore();