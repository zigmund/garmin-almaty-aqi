import { beforeEach, expect, test } from "vitest";
import { __resetHeatmapCacheForTests, buildHeatmap } from "../src/heatmap";
import { UpstreamError, type MapSnapshot } from "../src/aai";
import { CITIES } from "../src/cities";

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
