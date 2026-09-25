import {
  ProxyGetRequest,
  ProxyPostRequest,
} from "../../../wailsjs/go/main/App";

// export const API_URL = "http://localhost:3000";
// export const API_URL = "http://192.168.100.224:8087";
export const API_URL = "http://192.168.17.17:8087";

/**
 * Safely parse a JSON string. If parsing fails the error message includes
 * the API name and a preview of the raw response so you can tell whether
 * the backend returned HTML (e.g. a 502 error page) instead of JSON.
 * @param {string} raw - Raw response body
 * @param {string} apiName - Human-readable label for error messages
 * @returns {any} Parsed JSON value
 */
function safeJsonParse(raw, apiName) {
  try {
    return JSON.parse(raw);
  } catch (e) {
    const preview = raw.length > 150 ? raw.slice(0, 150) + "..." : raw;
    throw new Error(
      `[${apiName}] Invalid JSON response (first 150 chars): ${preview}`,
    );
  }
}

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
      power: Number(dataArray[3].trim()),
      polar: dataArray.slice(17, 377).map(Number),
      // polar: dataArray.slice(17, 377).map(Number).reverse(),
    };
    return { success: true, data };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }
};

export const setAntenna = async (/** @type {number} */ antSpace) => {
  let typeAnt = "vhf";
  if (antSpace <= 0.25) {
    typeAnt = "uhf";
  }

  try {
    const response = await ProxyGetRequest(API_URL + "/api/ant/" + typeAnt);
    const jsonResponse = safeJsonParse(response, "setAntenna");
    return { success: true, data: jsonResponse };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }
};

export const setFreqGainApi = async (
  /** @type {{center_freq: number, uniform_gain: number, ant_spacing_meters: number}} */ data,
) => {
  try {
    const response = await ProxyPostRequest(
      `${API_URL}/api/settings/freq`,
      JSON.stringify(data),
    );
    const jsonResponse = safeJsonParse(response, "setFreqGain");
    return { success: true, data: jsonResponse };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }
};

export const readCompass = async () => {
  try {
    const response = await ProxyGetRequest(`${API_URL}/api/compass`);
    const data = safeJsonParse(response, "readCompass");
    return { success: true, data: Number(data.heading) };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }
};

export const getDFSettings = async () => {
  const response = await ProxyGetRequest(`${API_URL}/api/settings`);
  const result = safeJsonParse(response, "getDFSettings");
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
    const response = await ProxyPostRequest(
      API_URL + "/api/settings/station_id",
      JSON.stringify(stationId),
    );
    const jsonResponse = safeJsonParse(response, "setStationId");
    return { success: true, data: jsonResponse };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    };
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
    const json = safeJsonParse(response, "readGPSExternal");
    return { success: true, data: json.data ?? json };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }
};
