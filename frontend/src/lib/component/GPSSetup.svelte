<script lang="ts">
    import { locationStore } from "../store/locationStore.svelte.js";
    import * as utm from "utm";

    let lat = $state(locationStore.data.latitude);
    let lng = $state(locationStore.data.longitude);
    let utmZone = $state("");
    let utmEasting = $state("");
    let utmNorthing = $state("");
    let utmCO = $state("");

    function readGPS() {
        locationStore.fetchGPS();

        setTimeout(() => {
            lat = locationStore.data.latitude;
            lng = locationStore.data.longitude;
        }, 500);
    }

    function setGPS() {
        if (lat === null || lng === null) {
            lat = 0;
            lng = 0;
            return;
        }
        locationStore.set(lat, lng);
    }

    function convertUTM() {
        if (lat === null || lng === null) {
            return;
        }
        const utmResult = utm.fromLatLon(lat, lng);
        utmZone = utmResult.zoneNum + utmResult.zoneLetter;
        utmEasting = utmResult.easting.toFixed(2);
        utmNorthing = utmResult.northing.toFixed(2);

        const strCOE = Math.round(utmResult.easting).toString();
        const strCON = Math.round(utmResult.northing).toString();

        utmCO = `${strCOE.substring(1, strCOE.length - 1)}, ${strCON.substring(2, strCON.length - 1)}`;
    }
</script>

<div class="container">
    <div class="input-panel">
        <div class="latlng-content">
            <div class="latlng-field">
                <div>Latitude</div>
                <input type="number" bind:value={lat} />
            </div>
            <div class="latlng-field">
                <div>Longitude</div>
                <input type="number" bind:value={lng} />
            </div>
        </div>
        <div class="utm-content">
            <div class="utm-field">
                <div>Zone :</div>
                <input type="text" bind:value={utmZone} />
            </div>
            <div class="utm-field">
                <div>Northing :</div>
                <input type="text" bind:value={utmNorthing} />
            </div>
            <div class="utm-field">
                <div>Easting :</div>
                <input type="text" bind:value={utmEasting} />
            </div>
            <div class="utm-field">
                <div>CO :</div>
                <input type="text" bind:value={utmCO} />
            </div>
        </div>
    </div>
    <div class="button-panel">
        <button onclick={readGPS}>Read</button>
        <button onclick={setGPS}>Set</button>
        <button onclick={convertUTM}>Convert UTM</button>
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
    .latlng-field {
        padding: 2px 4px;
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
