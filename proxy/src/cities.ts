// Supported cities and their station bounding boxes.

export type CityCode = "almaty" | "karaganda";

export const CITIES: Record<CityCode, { bbox: [w: number, s: number, e: number, n: number] }> = {
  almaty: { bbox: [76.6, 43.14, 77.26, 43.5] },
  karaganda: { bbox: [72.9, 49.7, 73.3, 49.95] },
};

const MARGIN_KM = 15;
const KM_PER_DEG = 111.32;

export function isCity(x: string): x is CityCode {
  return Object.prototype.hasOwnProperty.call(CITIES, x);
}

// Cities whose bbox, expanded by 15 km, contains the point.
export function candidateCities(lat: number, lon: number): CityCode[] {
  const dLat = MARGIN_KM / KM_PER_DEG;
  return (Object.keys(CITIES) as CityCode[]).filter((c) => {
    const [w, s, e, n] = CITIES[c].bbox;
    const dLon = MARGIN_KM / (KM_PER_DEG * Math.cos((((s + n) / 2) * Math.PI) / 180));
    return lat >= s - dLat && lat <= n + dLat && lon >= w - dLon && lon <= e + dLon;
  });
}
