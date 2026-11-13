// /v1/glance response: nearest-station (near), city median (city) or out of coverage (out).

import { UpstreamError, dataAgeSec, type History, type MapSnapshot, type Station } from "./aai";
import { candidateCities, type CityCode } from "./cities";
import { idw, median, selectNear } from "./geo";
import { pmToAqi } from "./aqi";

export type Resolved =
  | { mode: "near"; city: CityCode; near: { s: Station; d: number }[] }
  | { mode: "city"; city: CityCode }
  | { mode: "out" };

export type GlanceResponse =
  | { m: "out" }
  | { m: "near" | "city"; c: CityCode; aqi: number; pm: number; h: (number | null)[]; t: string; st: number; fb: boolean; a?: number };

const HOUR_MS = 3_600_000;
const HISTORY_HOURS = 5;

export function resolve(
  maps: Partial<Record<CityCode, MapSnapshot>>,
  pos: { lat: number; lon: number } | null,
  lastCity: CityCode,
): Resolved {
  if (pos == null) return { mode: "city", city: lastCity };
  for (const city of candidateCities(pos.lat, pos.lon)) {
    const map = maps[city];
    if (!map) continue;
    const near = selectNear(map.stations, pos.lat, pos.lon);
    if (near.length > 0) return { mode: "near", city, near };
  }
  return { mode: "out" };
}

export function buildGlance(r: Resolved, map: MapSnapshot | null, hist: History | null, nowMs: number = Date.now()): GlanceResponse {
  if (r.mode === "out") return { m: "out" };
  if (!map || map.stations.length === 0) throw new UpstreamError("no map data");

  // value(hourPm) returns the aggregated PM for one hour, or null if no station has data.
  let value: (pm: (id: number) => number | undefined) => number | null;
  let pm: number;
  let st: number;
  if (r.mode === "near") {
    const near = r.near;
    st = near.length;
    value = (get) => {
      const pts: { d: number; v: number }[] = [];
      for (const { s, d } of near) {
        const v = get(s.id);
        if (v !== undefined) pts.push({ d, v });
      }
      return pts.length > 0 ? idw(pts) : null;
    };
    pm = idw(near.map(({ s, d }) => ({ d, v: s.pm })));
  } else {
    st = map.stations.length;
    value = (get) => {
      const vs: number[] = [];
      for (const s of map.stations) {
        const v = get(s.id);
        if (v !== undefined) vs.push(v);
      }
      return median(vs);
    };
    pm = median(map.stations.map((s) => s.pm)) as number;
  }

  const aqi = pmToAqi(pm);
  const end = Date.parse(map.windowEnd);
  const closed = hist ? hist.hours.filter((x) => Date.parse(x.hour) + HOUR_MS <= end).slice(-HISTORY_HOURS) : [];
  const h: (number | null)[] = closed.map((x) => {
    const v = value((id) => x.pm.get(id));
    return v === null ? null : pmToAqi(v);
  });
  while (h.length < HISTORY_HOURS) h.unshift(null);
  h.push(aqi);

  const a = dataAgeSec(map.windowEnd, nowMs);
  return {
    m: r.mode,
    c: r.city,
    aqi,
    pm: Math.round(pm * 10) / 10,
    h,
    t: map.windowEnd,
    st,
    fb: map.fallback,
    ...(a === undefined ? {} : { a }),
  };
}
