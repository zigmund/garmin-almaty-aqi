// Reproducible CPU benchmark of the heatmap grid: `npm run bench`.
// 153 synthetic stations over the Almaty bbox (the real network size); every iteration
// recomputes the 24x24 grid (a fresh windowEnd defeats the per-city cache).
import { buildHeatmap, __resetHeatmapCacheForTests } from "../src/heatmap";
import { CITIES } from "../src/cities";

const [W, S, E, N] = CITIES.almaty.bbox;
let seed = 12345;
const rnd = () => (seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296;
const stations = Array.from({ length: 153 }, (_, i) => ({
  id: i,
  lat: S + rnd() * (N - S),
  lon: W + rnd() * (E - W),
  pm: rnd() * 80,
}));

const run = (i: number) => {
  __resetHeatmapCacheForTests();
  const t = performance.now();
  buildHeatmap({ mode: "city", city: "almaty" }, { stations, windowEnd: `w${i}`, fallback: false }, null);
  return performance.now() - t;
};

const cold = run(0);
const warm = Array.from({ length: 200 }, (_, i) => run(i + 1)).sort((a, b) => a - b);
console.log(`cold ${cold.toFixed(2)} ms, warm median ${warm[100].toFixed(2)} ms, p95 ${warm[190].toFixed(2)} ms`);
