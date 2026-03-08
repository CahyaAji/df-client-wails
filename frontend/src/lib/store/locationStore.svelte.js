
import { readGPSExternal } from "../utils/api_handler";

class LocationStore {
    data = $state(/** @type {{ latitude: number | null, longitude: number | null }} */ ({ latitude: null, longitude: null }));
    error = $state(/** @type {string | null} */ (null));
    isLoading = $state(false);
    /** @type {'gps' | 'manual' | null} */
    source = $state(null);

    /** Fetch location from the browser's Geolocation API */
    fetchGPS() {
        this.isLoading = true;
        this.error = null;

        if (!("geolocation" in navigator)) {
            this.error = "Geolocation is not supported by this App";
            this.isLoading = false;
            return;
        }

        navigator.geolocation.getCurrentPosition(
            (position) => {
                this.data = {
                    latitude: parseFloat(position.coords.latitude.toFixed(6)),
                    longitude: parseFloat(position.coords.longitude.toFixed(6)),
                };
                this.source = "gps";
                this.isLoading = false;
            },
            (err) => {
                this.error = err.message;
                this.isLoading = false;
            },
            // Don't block startup: give up after 5 s, accept a cached fix up to
            // 5 minutes old, and skip the slow high-accuracy (GPS) path.
            { timeout: 5000, maximumAge: 300000, enableHighAccuracy: false }
        );
    }

    /** Fetch location from external IoT GPS endpoint */
    async fetchGPSExternal() {
        this.isLoading = true;
        this.error = null;

        const result = await readGPSExternal();

        if (!result.success) {
            this.fetchGPS();
            return;
        }

        const payload = result.data ?? {};
        const rawLat = payload.lat ?? payload.latitude;
        const rawLng = payload.lng ?? payload.longitude;
        const latitude = Number(rawLat);
        const longitude = Number(rawLng);

        if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
            this.fetchGPS();
            return;
        }

        this.data = {
            latitude: parseFloat(latitude.toFixed(6)),
            longitude: parseFloat(longitude.toFixed(6)),
        };
        this.source = "gps";
        this.isLoading = false;
    }

    /**
     * Manually set the location.
     * @param {number} latitude
     * @param {number} longitude
     */
    set(latitude, longitude) {
        this.data = { latitude, longitude };
        this.source = "manual";
        this.error = null;
    }
}

export const locationStore = new LocationStore();