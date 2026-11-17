import { beforeEach, expect, test } from "vitest";
import { __resetHeatmapCacheForTests, buildHeatmap } from "../src/heatmap";
import { UpstreamError, type MapSnapshot } from "../src/aai";
import { CITIES } from "../src/cities";
import { aqiCategory, pmToAqi } from "../src/aqi";
import { distKm, idw } from "../src/geo";

const [W, S, E, N] = CITIES.almaty.bbox;
const CENTER = { lat: (S + N) / 2, lon: (W + E) / 2 };
const snap = (stations: MapSnapshot["stations"], windowEnd = "2026-11-14T03:00:00+00:00"): MapSnapshot => ({
  stations,
  windowEnd,
  fallback: false,
});
const near = { mode: "city", city: "almaty" } as const;
const bytes = (g: string) => Uint8Array.from(atob(g), (c) => c.charCodeAt(0));

beforeEach(() => __resetHeatmapCacheForTests());

test("out of coverage", () => {
  expect(buildHeatmap({ mode: "out" }, snap([]), null)).toEqual({ m: "out" });
});

test("empty map throws UpstreamError", () => {
  expect(() => buildHeatmap(near, snap([]), null)).toThrow(UpstreamError);
});

test("single central station: centre cell category, corners no data, 576 bytes", () => {
  const r = buildHeatmap(near, snap([{ id: 1, ...CENTER, pm: 60 }]), null);
  if (r.m === "out") throw new Error("unexpected");
  const b = bytes(r.g);
  expect(r.w).toBe(24);
  expect(r.h).toBe(24);
  expect(b.length).toBe(576);
  expect(b[11 * 24 + 11]).toBe(3);
  for (const i of [0, 23, 23 * 24, 575]) expect(b[i]).toBe(255);
  expect(r.s).toEqual([[Number(CENTER.lat.toFixed(4)), Number(CENTER.lon.toFixed(4)), 3]]);
  expect(r.t).toBe("2026-11-14T03:00:00+00:00");
  expect(r.bbox).toEqual([W, S, E, N]);
});

test("station near NW corner lands in byte 0", () => {
  const lon = W + 0.5 * ((E - W) / 24);
  const lat = N - 0.5 * ((N - S) / 24);
  const r = buildHeatmap(near, snap([{ id: 1, lat, lon, pm: 5 }]), null);
  if (r.m === "out") throw new Error("unexpected");
  expect(bytes(r.g)[0]).toBe(0);
});

test("me: centre, outside bbox, null", () => {
  const map = snap([{ id: 1, ...CENTER, pm: 60 }]);
  const a = buildHeatmap(near, map, CENTER);
  if (a.m === "out") throw new Error("unexpected");
  expect(a.me).toEqual([0.5, 0.5]);
  const b = buildHeatmap(near, map, { lat: 50, lon: 60 });
  if (b.m === "out") throw new Error("unexpected");
  expect("me" in b).toBe(false);
  const c = buildHeatmap(near, map, null);
  if (c.m === "out") throw new Error("unexpected");
  expect("me" in c).toBe(false);
});

test("same windowEnd reuses cached grid; new windowEnd recomputes", () => {
  const stations = [{ id: 1, ...CENTER, pm: 60 }];
  const a = buildHeatmap(near, snap(stations), null);
  const b = buildHeatmap(near, snap(stations), CENTER);
  const c = buildHeatmap(near, snap(stations, "2026-11-14T04:00:00+00:00"), null);
  if (a.m === "out" || b.m === "out" || c.m === "out") throw new Error("unexpected");
  expect(b.s).toBe(a.s);
  expect(c.s).not.toBe(a.s);
});

// Straightforward reference (full sort per cell): the grid must stay byte-identical to it.
function referenceGrid(stations: MapSnapshot["stations"], size: number): Uint8Array {
  const out = new Uint8Array(size * size);
  for (let row = 0; row < size; row++) {
    const lat = N - ((row + 0.5) * (N - S)) / size;
    for (let col = 0; col < size; col++) {
      const lon = W + ((col + 0.5) * (E - W)) / size;
      const nearest = stations
        .map((st) => ({ d: distKm(lat, lon, st.lat, st.lon), v: st.pm }))
        .sort((a, b) => a.d - b.d)
        .slice(0, 8);
      out[row * size + col] = nearest.length === 0 || nearest[0].d > 3 ? 255 : aqiCategory(pmToAqi(idw(nearest)));
    }
  }
  return out;
}

test("grid equals the reference on 153 pseudo-random stations", () => {
  let seed = 7;
  const rnd = () => (seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296;
  const stations = Array.from({ length: 153 }, (_, id) => ({
    id,
    lat: S + rnd() * (N - S),
    lon: W + rnd() * (E - W),
    pm: rnd() * 150,
  }));
  const r = buildHeatmap(near, snap(stations), null);
  if (r.m === "out") throw new Error("unexpected");
  expect(Array.from(bytes(r.g))).toEqual(Array.from(referenceGrid(stations, 24)));
});

test("grid equals the reference with ties and fewer than 8 stations", () => {
  const stations = [
    { id: 1, lat: CENTER.lat + 0.01, lon: CENTER.lon, pm: 10 },
    { id: 2, lat: CENTER.lat - 0.01, lon: CENTER.lon, pm: 90 },
    { id: 3, lat: CENTER.lat, lon: CENTER.lon, pm: 40 },
  ];
  const r = buildHeatmap(near, snap(stations), null, 12);
  if (r.m === "out") throw new Error("unexpected");
  expect(Array.from(bytes(r.g))).toEqual(Array.from(referenceGrid(stations, 12)));
});

test("a: age of the data in seconds", () => {
  const t = Date.parse("2026-11-14T03:00:00+00:00");
  const r = buildHeatmap(near, snap([{ id: 1, ...CENTER, pm: 60 }]), null, 24, t + 90_000);
  if (r.m === "out") throw new Error("unexpected");
  expect(r.a).toBe(90);
  const r2 = buildHeatmap(near, snap([{ id: 1, ...CENTER, pm: 60 }]), null, 24, t - 5_000);
  if (r2.m === "out") throw new Error("unexpected");
  expect(r2.a).toBe(0);
});

test("a: not cached with the grid", () => {
  const t = Date.parse("2026-11-14T03:00:00+00:00");
  const m = snap([{ id: 1, ...CENTER, pm: 60 }]);
  const a = buildHeatmap(near, m, null, 24, t + 10_000);
  const b = buildHeatmap(near, m, null, 24, t + 200_000);
  if (a.m === "out" || b.m === "out") throw new Error("unexpected");
  expect([a.a, b.a]).toEqual([10, 200]);
});
