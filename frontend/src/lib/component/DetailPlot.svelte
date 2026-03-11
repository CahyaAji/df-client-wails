<script>
  import { dfStore } from "../store/dfStore.svelte.js";
  import { configStore } from "../store/configStore.svelte.js";
  import { signalState } from "../store/signalState.svelte.js";

  /**
   * @param {unknown} rawData
   */
  function normalizePolarData(rawData) {
    if (!Array.isArray(rawData)) return [];

    return rawData
      .map((value) => Number(value))
      .filter((value) => Number.isFinite(value));
  }

  /**
   * @param {number[]} data
   * @param {number} radiusSize
   */
  function smoothCircularData(data, radiusSize = 2) {
    if (!Array.isArray(data) || data.length === 0 || radiusSize <= 0) {
      return data;
    }

    const length = data.length;
    return data.map((_, index) => {
      let sum = 0;
      let count = 0;

      for (let offset = -radiusSize; offset <= radiusSize; offset++) {
        const wrappedIndex = (index + offset + length) % length;
        sum += data[wrappedIndex];
        count++;
      }

      return count > 0 ? sum / count : data[index];
    });
  }

  let polarData = $derived(normalizePolarData(dfStore.data?.polar));
  let smoothedPolarData = $derived(smoothCircularData(polarData, 2));

  const width = 210;
  const height = 210;
  const centerX = width / 2;
  const centerY = height / 2;
  const radius = 100;

  /**
   * @param {number[]} data
   */
  function createRadarPath(data) {
    if (data.length === 0) return "";

    const maxValue = Math.max(...data);
    const minValue = Math.min(...data);
    const range = maxValue - minValue || 1; // Avoid division by zero

    const points = data.map((value, index) => {
      const angle =
        ((index * 360) / data.length) * (Math.PI / 180) - Math.PI / 2;
      const normalizedValue = (value - minValue) / range;
      const r = normalizedValue * radius;
      return {
        x: centerX + r * Math.cos(angle),
        y: centerY + r * Math.sin(angle),
      };
    });

    if (points.length < 3) {
      let fallbackPath = "";
      points.forEach((point, index) => {
        if (index === 0) {
          fallbackPath += `M ${point.x} ${point.y}`;
        } else {
          fallbackPath += ` L ${point.x} ${point.y}`;
        }
      });
      return `${fallbackPath} Z`;
    }

    let path = `M ${points[0].x} ${points[0].y}`;

    for (let i = 0; i < points.length; i++) {
      const prev = points[(i - 1 + points.length) % points.length];
      const current = points[i];
      const next = points[(i + 1) % points.length];
      const nextNext = points[(i + 2) % points.length];

      const cp1x = current.x + (next.x - prev.x) / 6;
      const cp1y = current.y + (next.y - prev.y) / 6;
      const cp2x = next.x - (nextNext.x - current.x) / 6;
      const cp2y = next.y - (nextNext.y - current.y) / 6;

      path += ` C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${next.x} ${next.y}`;
    }

    return `${path} Z`;
  }

  // Create grid circles
  function createGridCircles() {
    const circles = [];
    for (let i = 1; i <= 4; i++) {
      circles.push({
        r: (radius / 4) * i,
        opacity: 0.3,
      });
    }
    return circles;
  }

  let gridCircles = createGridCircles();
  let radarPath = $derived(createRadarPath(smoothedPolarData));
</script>

<div
  class="chart-wrapper"
  style={dfStore.data &&
  dfStore.data.heading !== undefined &&
  dfStore.data.heading !== null
    ? signalState.currentFreq < 250
      ? `transform: rotate(${configStore.offsetVhf}deg);`
      : `transform: rotate(${configStore.offsetUhf}deg);`
    : ""}
>
  {#if radarPath}
    <svg {width} {height} class="radar-svg">
      <!-- Grid circles -->
      {#each gridCircles as circle}
        <circle
          cx={centerX}
          cy={centerY}
          r={circle.r}
          fill="none"
          stroke="white"
          stroke-width="1"
          opacity={circle.opacity}
        />
      {/each}

      <!-- Radar data -->
      <path
        d={radarPath}
        fill="rgba(0, 50, 255, 0.3)"
        stroke="rgba(0, 50, 255, 1)"
        stroke-width="1"
        stroke-linejoin="round"
        stroke-linecap="round"
      />
    </svg>
  {:else}
    <div class="no-data">
      <div>No DF Data</div>
      <div class="status">
        {#if dfStore.isLoading}
          Loading...
        {:else if dfStore.error}
          Error: {dfStore.error}
        {:else}
          Waiting for data...
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .chart-wrapper {
    width: 210px;
    height: 210px;
    margin: auto;
    border-radius: 50%;
    background-color: black;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    position: relative;
    pointer-events: none;
  }

  .radar-svg {
    background-color: transparent;
    pointer-events: none;
  }

  .no-data {
    color: white;
    text-align: center;
  }

  .no-data > div:first-child {
    font-size: 18px;
    font-weight: bold;
    margin-bottom: 8px;
  }

  .status {
    font-size: 14px;
    opacity: 0.7;
  }
</style>
