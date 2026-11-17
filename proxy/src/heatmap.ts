// /v1/heatmap response: a size x size grid of AQI categories over the city bbox plus station dots.

import type { MapSnapshot } from "./aai";
import { UpstreamError, dataAgeSec } from "./aai";
import { aqiCategory, pmToAqi, type Category } from "./aqi";
import { CITIES, type CityCode } from "./cities";
import { idw } from "./geo";
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
      a?: number;
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

const KM_PER_DEG = 111.32;
const RAD = Math.PI / 180;

// Same maths as geo.distKm (equirectangular, cos of the pair's mean latitude) but cheap:
// cos((a+b)/2) = cos(a/2)cos(b/2) - sin(a/2)sin(b/2), with the station terms computed once
// and the cell terms once per row. The K nearest stay in a small insertion-sorted buffer
// (ties keep station order, like a stable sort). Runs in a Worker with a ~10 ms CPU limit.
function compute(city: CityCode, map: MapSnapshot, size: number): Entry {
  const [w, s, e, n] = CITIES[city].bbox;
  const st = map.stations;
  const count = st.length;
  const slat = new Float64Array(count);
  const slon = new Float64Array(count);
  const spm = new Float64Array(count);
  const sCos = new Float64Array(count);
  const sSin = new Float64Array(count);
  for (let i = 0; i < count; i++) {
    slat[i] = st[i].lat;
    slon[i] = st[i].lon;
    spm[i] = st[i].pm;
    sCos[i] = Math.cos(st[i].lat * RAD * 0.5);
    sSin[i] = Math.sin(st[i].lat * RAD * 0.5);
  }

  const bd = new Float64Array(K_NEAREST); // squared distances in degrees-ish units, ascending
  const bv = new Float64Array(K_NEAREST);
  const bytes = new Array<number>(size * size);
  const pts: { d: number; v: number }[] = [];
  for (let row = 0; row < size; row++) {
    const lat = n - ((row + 0.5) * (n - s)) / size;
    const rCos = Math.cos(lat * RAD * 0.5);
    const rSin = Math.sin(lat * RAD * 0.5);
    for (let col = 0; col < size; col++) {
      const lon = w + ((col + 0.5) * (e - w)) / size;
      let k = 0;
      for (let i = 0; i < count; i++) {
        const x = (slon[i] - lon) * (rCos * sCos[i] - rSin * sSin[i]);
        const y = slat[i] - lat;
        const d2 = x * x + y * y;
        let j: number;
        if (k < K_NEAREST) j = k++;
        else if (d2 < bd[K_NEAREST - 1]) j = K_NEAREST - 1;
        else continue;
        while (j > 0 && bd[j - 1] > d2) {
          bd[j] = bd[j - 1];
          bv[j] = bv[j - 1];
          j--;
        }
        bd[j] = d2;
        bv[j] = spm[i];
      }
      if (k === 0 || KM_PER_DEG * Math.sqrt(bd[0]) > NO_DATA_KM) {
        bytes[row * size + col] = NO_DATA;
        continue;
      }
      pts.length = 0;
      for (let j = 0; j < k; j++) pts.push({ d: KM_PER_DEG * Math.sqrt(bd[j]), v: bv[j] });
      bytes[row * size + col] = aqiCategory(pmToAqi(idw(pts)));
    }
  }
  return {
    windowEnd: map.windowEnd,
    size,
    g: btoa(String.fromCharCode(...bytes)),
    s: st.map((x) => [round(x.lat, 4), round(x.lon, 4), aqiCategory(pmToAqi(x.pm))]),
  };
}

export function buildHeatmap(
  r: Resolved,
  map: MapSnapshot,
  pos: { lat: number; lon: number } | null,
  size = 24,
  nowMs: number = Date.now(),
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
  const a = dataAgeSec(map.windowEnd, nowMs);
  if (a !== undefined) res.a = a;
  if (pos && pos.lat >= s && pos.lat <= n && pos.lon >= w && pos.lon <= e) {
    res.me = [round((pos.lon - w) / (e - w), 3), round((n - pos.lat) / (n - s), 3)];
  }
  return res;
}
