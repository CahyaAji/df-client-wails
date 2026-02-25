
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
                    latitude: position.coords.latitude,
                    longitude: position.coords.longitude,
                };
                this.source = "gps";
                this.isLoading = false;
            },
            (err) => {
                this.error = err.message;
                this.isLoading = false;
            }
        );
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

    /** Clear the stored location */
    clear() {
        this.data = { latitude: null, longitude: null };
        this.source = null;
        this.error = null;
    }
}

export const locationStore = new LocationStore();