// export const API_URL = "http://localhost:3000";
export const API_URL = "http://192.168.17.17:8087";

// All requests to the DF device use a short timeout so a missing/unreachable
// device never stalls startup for more than FETCH_TIMEOUT_MS milliseconds.
const FETCH_TIMEOUT_MS = 3000;

/**
 * fetch() wrapper that aborts after FETCH_TIMEOUT_MS milliseconds.
 * @param {string} url
 * @param {RequestInit} [options]
 * @returns {Promise<Response>}
 */
function fetchWithTimeout(url, options = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  return fetch(url, { ...options, signal: controller.signal })
    .finally(() => clearTimeout(timer));
}

export const readDF = async () => {
  try {
    const response = await fetchWithTimeout(`${API_URL}/df`);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const resText = await response.text();
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
    const response = await fetchWithTimeout(API_URL + "/api/ant/" + typeAnt, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    if (response.status === 200) {
      const jsonResponse = await response.json();
      return { success: true, data: jsonResponse };
    } else {
      const errorText = await response.text();
      return { success: false, error: `${response.status}: ${errorText}` };
    }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const setFreqGainApi = async (
  /** @type {{center_freq: number, uniform_gain: number, ant_spacing_meters: number}} */ data
) => {
  try {
    const response = await fetchWithTimeout(`${API_URL}/api/settings/freq`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(data),
    });

    if (response.ok) {
      const jsonResponse = await response.json();
      return { success: true, data: jsonResponse };
    } else {
      const errorText = await response.text();
      return { success: false, error: `HTTP ${response.status}: ${errorText}` };
    }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const readCompass = async () => {
  try {
    const response = await fetchWithTimeout(`${API_URL}/api/compass`);

    if (!response.ok) {
      const errorText = await response.text();
      return { success: false, error: `HTTP ${response.status}: ${errorText}` };
    }
    const data = await response.json();
    return { success: true, data: Number(data.heading) };
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const getDFSettings = async () => {
  const response = await fetchWithTimeout(`${API_URL}/api/settings`);
  const result = await response.json();
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
    const response = await fetchWithTimeout(API_URL + "/api/settings/station_id", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(stationId),
    });
    if (response.status === 200) {
      const jsonResponse = await response.json();
      return { success: true, data: jsonResponse };
    } else {
      const errorText = await response.text();
      return { success: false, error: `${response.status}: ${errorText}` };
    }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
};

export const turnOffDf = async () => {
  try {
    await fetchWithTimeout(API_URL + "/api/shutdown", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
    });
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
    await fetchWithTimeout(API_URL + "/api/restart", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
    });
  } catch (error) {
    console.error("Error RestartDF: ", error);
  } finally {
    setTimeout(() => {
      console.log("restarting DF App");
    }, 2000);
  }
};
