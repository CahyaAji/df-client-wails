export const API_URL = "http://localhost:3000";
// export const API_URL = "http://192.168.17.17:8087";

export const readDF = async () => {
  try {
    const response = await fetch(`${API_URL}/df`);
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