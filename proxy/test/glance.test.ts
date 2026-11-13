import { expect, test } from "vitest";
import { buildGlance, resolve } from "../src/glance";
import { UpstreamError, type History, type MapSnapshot } from "../src/aai";
import { pmToAqi } from "../src/aqi";
import { idw, median } from "../src/geo";

const POS = { lat: 43.25, lon: 76.9 };
const KM = 111.32;
const north = (id: number, km: number, pm: number) => ({ id, lat: POS.lat + km / KM, lon: POS.lon, pm });
const WINDOW_END = "2026-11-14T03:00:00+00:00";
const snap = (stations: MapSnapshot["stations"], fallback = false): MapSnapshot => ({
  stations,
  windowEnd: WINDOW_END,
  fallback,
});
const hour = (h: number) => `2026-11-14T${String(h).padStart(2, "0")}:00:00+00:00`;
const hist = (hs: [number, [number, number][]][]): History => ({
  hours: hs.map(([h, pms]) => ({ hour: hour(h), pm: new Map(pms) })),
});

const three = snap([north(1, 1, 10), north(2, 2, 20), north(3, 4, 40)]);

test("near: IDW over three stations", () => {
  const r = resolve({ almaty: three }, POS, "almaty");
  expect(r.mode).toBe("near");
  const g = buildGlance(r, three, null);
  const pm = idw([{ d: 1, v: 10 }, { d: 2, v: 20 }, { d: 4, v: 40 }]);
  expect(g).toMatchObject({ m: "near", c: "almaty", st: 3, pm: Math.round(pm * 10) / 10, aqi: pmToAqi(pm), fb: false, t: WINDOW_END });
});

test("near: single station at 10 km", () => {
  const m = snap([north(1, 10, 12)]);
  const r = resolve({ almaty: m }, POS, "almaty");
  const g = buildGlance(r, m, null);
  expect(g).toMatchObject({ m: "near", st: 1, pm: 12 });
});

test("city: median over all stations, lastCity", () => {
  const m = snap([north(1, 1, 10), north(2, 2, 20), north(3, 4, 40), north(4, 5, 50)], true);
  const r = resolve({ karaganda: m }, null, "karaganda");
  expect(r).toEqual({ mode: "city", city: "karaganda" });
  const g = buildGlance(r, m, null);
  expect(g).toMatchObject({ m: "city", c: "karaganda", pm: 30, aqi: pmToAqi(30), st: 4, fb: true });
});

test("out: Astana", () => {
  const r = resolve({ almaty: three }, { lat: 51.13, lon: 71.43 }, "almaty");
  expect(r).toEqual({ mode: "out" });
  expect(buildGlance(r, null, null)).toEqual({ m: "out" });
});

test("resolve skips candidate city without map", () => {
  expect(resolve({}, POS, "almaty")).toEqual({ mode: "out" });
});

test("h: five latest closed hours, open hour excluded, last = current aqi", () => {
  const m = snap([north(1, 1, 10)]);
  const r = resolve({ almaty: m }, POS, "almaty");
  const h = {
    hours: [
      { hour: "2026-11-13T20:00:00+00:00", pm: new Map([[1, 1]]) },
      { hour: "2026-11-13T21:00:00+00:00", pm: new Map([[1, 11]]) },
      { hour: "2026-11-13T22:00:00+00:00", pm: new Map([[1, 21]]) },
      { hour: "2026-11-13T23:00:00+00:00", pm: new Map([[1, 31]]) },
      { hour: hour(0), pm: new Map([[1, 41]]) },
      { hour: hour(1), pm: new Map([[1, 51]]) },
      { hour: hour(2), pm: new Map([[1, 61]]) },
      { hour: hour(3), pm: new Map([[1, 99]]) }, // still open
    ],
  };
  const g = buildGlance(r, m, h);
  if (g.m === "out") throw new Error("unexpected");
  expect(g.h).toEqual([21, 31, 41, 51, 61].map(pmToAqi).concat([g.aqi]));
});

test("history gap yields null and h has length 6", () => {
  const m = snap([north(1, 1, 10), north(2, 2, 20)]);
  const r = resolve({ almaty: m }, POS, "almaty");
  const h = hist([
    [0, [[1, 10], [2, 20]]],
    [1, [[99, 50]]], // no selected station
    [2, [[1, 30]]],
    [3, [[1, 77]]], // open
  ]);
  const g = buildGlance(r, m, h);
  if (g.m === "out") throw new Error("unexpected");
  const a1 = pmToAqi(idw([{ d: 1, v: 10 }, { d: 2, v: 20 }]));
  expect(g.h).toEqual([null, null, a1, null, pmToAqi(30), g.aqi]);
  expect(g.h).toHaveLength(6);
});

test("city history uses median", () => {
  const m = snap([north(1, 1, 10), north(2, 2, 20), north(3, 3, 30)]);
  const h = hist([[2, [[1, 10], [2, 40], [3, 100]]], [3, []]]);
  const g = buildGlance({ mode: "city", city: "almaty" }, m, h);
  if (g.m === "out") throw new Error("unexpected");
  expect(g.h[4]).toBe(pmToAqi(median([10, 40, 100])!));
});

test("null history gives five nulls", () => {
  const g = buildGlance({ mode: "city", city: "almaty" }, three, null);
  if (g.m === "out") throw new Error("unexpected");
  expect(g.h.slice(0, 5)).toEqual([null, null, null, null, null]);
  expect(g.h).toHaveLength(6);
});

test("missing map throws UpstreamError", () => {
  expect(() => buildGlance({ mode: "city", city: "almaty" }, null, null)).toThrow(UpstreamError);
  expect(() => buildGlance({ mode: "city", city: "almaty" }, snap([]), null)).toThrow(UpstreamError);
});
