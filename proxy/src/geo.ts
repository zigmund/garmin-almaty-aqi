// Distance, inverse-distance weighting and nearest-station selection.

const KM_PER_DEG = 111.32;
const NEAR_KM = 5;
const FAR_KM = 15;
const MAX_NEAR = 3;

// Equirectangular approximation; accurate enough for distances up to ~50 km.
export function distKm(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const meanLat = ((lat1 + lat2) / 2) * (Math.PI / 180);
  const x = (lon2 - lon1) * Math.cos(meanLat);
  const y = lat2 - lat1;
  return KM_PER_DEG * Math.sqrt(x * x + y * y);
}

// Inverse-distance weighting with weight 1/d^2. Exact value if a point is at d ~ 0.
export function idw(pts: { d: number; v: number }[]): number {
  let num = 0;
  let den = 0;
  for (const { d, v } of pts) {
    if (d < 0.001) return v;
    const w = 1 / (d * d);
    num += w * v;
    den += w;
  }
  return num / den;
}

export function median(xs: number[]): number | null {
  if (xs.length === 0) return null;
  const s = [...xs].sort((a, b) => a - b);
  const m = s.length >> 1;
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
}

// Up to 3 nearest stations within 5 km; otherwise the single nearest within 15 km; otherwise [].
export function selectNear<T extends { lat: number; lon: number }>(
  stations: T[],
  lat: number,
  lon: number,
): { s: T; d: number }[] {
  const all = stations
    .map((s) => ({ s, d: distKm(lat, lon, s.lat, s.lon) }))
    .sort((a, b) => a.d - b.d);
  const near = all.filter((x) => x.d <= NEAR_KM).slice(0, MAX_NEAR);
  if (near.length > 0) return near;
  return all.length > 0 && all[0].d <= FAR_KM ? [all[0]] : [];
}
