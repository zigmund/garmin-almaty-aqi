import { expect, test } from "vitest";
import { distKm, idw, median, selectNear } from "../src/geo";

test('distKm 0.1 deg lon at 43.25N', () => {
  expect(distKm(43.25, 76.9, 43.25, 77.0)).toBeCloseTo(8.1, 1);
});

test('idw equal distances averages', () => {
  expect(idw([{ d: 1, v: 10 }, { d: 1, v: 20 }])).toBeCloseTo(15, 9);
});

test('idw weights by 1/d^2', () => {
  expect(idw([{ d: 1, v: 10 }, { d: 2, v: 20 }])).toBeCloseTo(12, 9);
});

test('idw returns exact value at zero distance', () => {
  expect(idw([{ d: 0, v: 7 }, { d: 1, v: 100 }])).toBe(7);
});

test('median', () => {
  expect(median([3, 1, 2])).toBe(2);
  expect(median([1, 2, 3, 4])).toBe(2.5);
  expect(median([])).toBeNull();
});

// 1 km north is ~0.009 deg lat
const st = (id: number, km: number) => ({ id, lat: 43.25 + km / 111.32, lon: 76.9 });

test('selectNear: up to 3 within 5 km, ascending', () => {
  const r = selectNear([st(4, 4), st(2, 2), st(1, 1), st(3, 3)], 43.25, 76.9);
  expect(r.map((x) => x.s.id)).toEqual([1, 2, 3]);
  expect(r[0].d).toBeCloseTo(1, 2);
});

test('selectNear: fallback to single nearest within 15 km', () => {
  const r = selectNear([st(1, 10)], 43.25, 76.9);
  expect(r.map((x) => x.s.id)).toEqual([1]);
});

test('selectNear: nothing within 15 km', () => {
  expect(selectNear([st(1, 16)], 43.25, 76.9)).toEqual([]);
});
