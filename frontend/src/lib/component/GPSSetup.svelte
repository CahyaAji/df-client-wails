<script lang="ts">
    function readGPS() {
        console.log("read GPS");
        if (!("geolocation" in navigator)) {
            console.log("Geolocation is not supported by this browser.");
            return;
        }

        navigator.geolocation.getCurrentPosition(
            (position) => {
                console.log("Latitude:", position.coords.latitude);
                console.log("Longitude:", position.coords.longitude);
            },
            (error) => {
                console.error("Error getting GPS location:", error);
            },
        );
    }
</script>

<div class="container">
    <div class="input-panel">
        <div class="latlng-content">
            <div>
                <div>Latitude</div>
                <input type="number" />
            </div>
            <div>
                <div>Longitude</div>
                <input type="number" />
            </div>
        </div>
        <div class="utm-content">
            <div class="utm-field">
                <div>Zone :</div>
                <input type="text" />
            </div>
            <div class="utm-field">
                <div>Northing :</div>
                <input type="text" />
            </div>
            <div class="utm-field">
                <div>Easting :</div>
                <input type="text" />
            </div>
            <div class="utm-field">
                <div>CO :</div>
                <input type="text" />
            </div>
        </div>
    </div>
    <div class="button-panel">
        <button
            onclick={() => {
                readGPS();
            }}>Read</button
        >
        <button>Convert UTM</button>
        <button>Apply</button>
    </div>
</div>

<style>
    .container {
        display: flex;
        flex-direction: column;
        color: white;
    }
    .input-panel {
        flex: 1;
        display: grid;
        grid-template-columns: 2fr 3fr;
    }
    input {
        width: 100px;
    }
    .utm-content {
        display: flex;
        flex-direction: column;
        align-items: center;
    }
    .utm-field {
        display: flex;
        align-items: center;
        padding: 2px 4px;
    }
    .utm-field > div {
        align-self: flex-end;
        min-width: 65px;
    }
    .utm-field > input {
        max-width: 100px;
    }
    .button-panel {
        display: flex;
        justify-content: center;
        gap: 8px;
        padding: 4px 0;
        background-color: rgba(4, 61, 15, 0.3);
    }
</style>
