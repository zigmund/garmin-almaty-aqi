// /v1/heatmap response: a size x size grid of AQI categories over the city bbox plus station dots.

import type { MapSnapshot } from "./aai";
import { UpstreamError } from "./aai";
import { aqiCategory, pmToAqi, type Category } from "./aqi";
import { CITIES, type CityCode } from "./cities";
import { distKm, idw } from "./geo";
import type { Resolved } from "./glance";

export type HeatmapResponse =
  | { m: "out" }
  | {
      m: "near" | "city";
      c: CityCode;
      bbox: [number, number, number, number];
      w: number;
      h: number;
      g: string;
      s: [number, number, Category][];
      me?: [number, number];
      t: string;
    };

const NO_DATA = 255;
const NO_DATA_KM = 3;
const K_NEAREST = 8;

type Entry = { windowEnd: string; size: number; g: string; s: [number, number, Category][] };

// One entry per city keeps the cache bounded.
const cache = new Map<CityCode, Entry>();

export function __resetHeatmapCacheForTests(): void {
  cache.clear();
}

const round = (x: number, digits: number) => Number(x.toFixed(digits));

function compute(city: CityCode, map: MapSnapshot, size: number): Entry {
  const [w, s, e, n] = CITIES[city].bbox;
  const bytes: number[] = [];
  for (let row = 0; row < size; row++) {
    const lat = n - ((row + 0.5) * (n - s)) / size;
    for (let col = 0; col < size; col++) {
      const lon = w + ((col + 0.5) * (e - w)) / size;
      const nearest = map.stations
        .map((st) => ({ d: distKm(lat, lon, st.lat, st.lon), v: st.pm }))
        .sort((a, b) => a.d - b.d)
        .slice(0, K_NEAREST);
      bytes.push(nearest.length === 0 || nearest[0].d > NO_DATA_KM ? NO_DATA : aqiCategory(pmToAqi(idw(nearest))));
    }
  }
  return {
    windowEnd: map.windowEnd,
    size,
    g: btoa(String.fromCharCode(...bytes)),
    s: map.stations.map((st) => [round(st.lat, 4), round(st.lon, 4), aqiCategory(pmToAqi(st.pm))]),
  };
}

export function buildHeatmap(
  r: Resolved,
  map: MapSnapshot,
  pos: { lat: number; lon: number } | null,
  size = 24,
): HeatmapResponse {
  if (r.mode === "out") return { m: "out" };
  if (map.stations.length === 0) throw new UpstreamError("no map data");

  let entry = cache.get(r.city);
  if (!entry || entry.windowEnd !== map.windowEnd || entry.size !== size) {
    entry = compute(r.city, map, size);
    cache.set(r.city, entry);
  }

  const bbox = CITIES[r.city].bbox;
  const [w, s, e, n] = bbox;
  const res: HeatmapResponse = {
    m: r.mode,
    c: r.city,
    bbox: [w, s, e, n],
    w: size,
    h: size,
    g: entry.g,
    s: entry.s,
    t: map.windowEnd,
  };
  if (pos && pos.lat >= s && pos.lat <= n && pos.lon >= w && pos.lon <= e) {
    res.me = [round((pos.lon - w) / (e - w), 3), round((n - pos.lat) / (n - s), 3)];
  }
  return res;
}
