import { ProxyGetRequest, ProxyPostRequest } from "../../../wailsjs/go/main/App";

export const API_URL = "http://localhost:3000";
// export const API_URL = "http://192.168.17.7:8087";
// export const API_URL = "http://192.168.17.17:8087";

export const readDF = async () => {
  try {
    const resText = await ProxyGetRequest(`${API_URL}/df`);
    if (!resText || resText.trim() === "") {
      throw new Error("DF data is empty");
    }

    const dataArray = resText.split(",").map((v) => v.trim());

    if (dataArray.length < 377) {
      throw new Error("DF data is incomplete");
    }

    const data = {
      time: dataArray[0].trim(),
      heading: (360 - Number(dataArray[1].trim())) % 360,
      confidence: dataArray[2].trim(),
      power: dataArray[3].trim(),
      polar: dataArray.slice(17, 377).map(Number).reverse(),
    };
    return { success: true, data };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const setAntenna = async (/** @type {number} */ antSpace) => {
  let typeAnt = "vhf";
  if (antSpace <= 0.25) {
    typeAnt = "uhf";
  }

  try {
    const response = await ProxyGetRequest(API_URL + "/api/ant/" + typeAnt);
    const jsonResponse = JSON.parse(response);
    return { success: true, data: jsonResponse };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const setFreqGainApi = async (
  /** @type {{center_freq: number, uniform_gain: number, ant_spacing_meters: number}} */ data
) => {
  try {
    const response = await ProxyPostRequest(`${API_URL}/api/settings/freq`, JSON.stringify(data));
    const jsonResponse = JSON.parse(response);
    return { success: true, data: jsonResponse };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const readCompass = async () => {
  try {
    const response = await ProxyGetRequest(`${API_URL}/api/compass`);
    const data = JSON.parse(response);
    return { success: true, data: Number(data.heading) };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const getDFSettings = async () => {
  const response = await ProxyGetRequest(`${API_URL}/api/settings`);
  const result = JSON.parse(response);
  return {
    center_freq: result.center_freq,
    uniform_gain: result.uniform_gain,
    station_id: result.station_id,
  };
};

export const setStationId = async (/** @type {string} */ nameId) => {
  const stationId = {
    id: nameId,
  };
  try {
    const response = await ProxyPostRequest(API_URL + "/api/settings/station_id", JSON.stringify(stationId));
	const jsonResponse = JSON.parse(response);
    return { success: true, data: jsonResponse };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const turnOffDf = async () => {
  try {
    await ProxyPostRequest(API_URL + "/api/shutdown", "{}");
  } catch (error) {
    console.error("Error TurnOffDF: ", error);
  } finally {
    setTimeout(() => {
      console.log("turning off DF App");
    }, 2000);
  }
};

export const restartDf = async () => {
  try {
    await ProxyPostRequest(API_URL + "/api/restart", "{}");
  } catch (error) {
    console.error("Error RestartDF: ", error);
  } finally {
    setTimeout(() => {
      console.log("restarting DF App");
    }, 2000);
  }
};

export const readGPSExternal = async () => {
  try {
    const response = await ProxyGetRequest(`${API_URL}/api/gps/status`);
    const json = JSON.parse(response);
    return { success: true, data: json.data ?? json };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};