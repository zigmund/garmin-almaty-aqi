// US AQI from PM2.5 (EPA 2024 breakpoints).

export type Category = 0 | 1 | 2 | 3 | 4 | 5;

// [pmLow, pmHigh, aqiLow, aqiHigh]
const BREAKPOINTS: ReadonlyArray<readonly [number, number, number, number]> = [
  [0.0, 9.0, 0, 50],
  [9.1, 35.4, 51, 100],
  [35.5, 55.4, 101, 150],
  [55.5, 125.4, 151, 200],
  [125.5, 225.4, 201, 300],
  [225.5, 325.4, 301, 500],
];

export function pmToAqi(pm: number): number {
  if (!(pm > 0)) return 0;
  // Truncate to 0.1; the epsilon guards against float error (e.g. 35.4 * 10).
  const c = Math.floor(pm * 10 + 1e-9) / 10;
  for (const [cLo, cHi, iLo, iHi] of BREAKPOINTS) {
    if (c <= cHi) {
      return Math.round(((iHi - iLo) / (cHi - cLo)) * (c - cLo) + iLo);
    }
  }
  return 500;
}

export function aqiCategory(aqi: number): Category {
  if (aqi <= 50) return 0;
  if (aqi <= 100) return 1;
  if (aqi <= 150) return 2;
  if (aqi <= 200) return 3;
  if (aqi <= 300) return 4;
  return 5;
}
