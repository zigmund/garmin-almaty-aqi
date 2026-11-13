// Almaty Air Initiative API client with a 5-minute in-memory cache.
// The Cache API does not work on *.workers.dev, so the cache lives in the isolate.

import type { CityCode } from "./cities";

export interface Station {
  id: number;
  lat: number;
  lon: number;
  pm: number;
}
export interface MapSnapshot {
  stations: Station[];
  windowEnd: string;
  fallback: boolean;
}
export interface History {
  hours: { hour: string; pm: Map<number, number> }[];
}

export class UpstreamError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "UpstreamError";
  }
}

const TTL_MS = 300_000;
const TIMEOUT_MS = 8000;

const isNum = (v: unknown): v is number => typeof v === "number" && Number.isFinite(v);
const isRec = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

function dataArray(json: unknown): unknown[] {
  if (!isRec(json) || !Array.isArray(json.data)) throw new UpstreamError("unexpected upstream shape");
  return json.data;
}

export function parseMap(json: unknown): MapSnapshot {
  const data = dataArray(json);
  const meta = isRec((json as Record<string, unknown>).meta) ? ((json as Record<string, unknown>).meta as Record<string, unknown>) : {};
  const stations: Station[] = [];
  for (const r of data) {
    if (!isRec(r)) continue;
    if (isNum(r.location_id) && isNum(r.latitude) && isNum(r.longitude) && isNum(r.pm25_avg)) {
      stations.push({ id: r.location_id, lat: r.latitude, lon: r.longitude, pm: r.pm25_avg });
    }
  }
  return {
    stations,
    windowEnd: typeof meta.window_end === "string" ? meta.window_end : "",
    fallback: meta.is_fallback === true,
  };
}

export function parseHistory(json: unknown): History {
  const data = dataArray(json);
  const hours: History["hours"] = [];
  for (const e of data) {
    if (!isRec(e) || typeof e.hour !== "string" || !Array.isArray(e.locations)) continue;
    const pm = new Map<number, number>();
    for (const l of e.locations) {
      if (isRec(l) && isNum(l.location_id) && isNum(l.pm25_avg)) pm.set(l.location_id, l.pm25_avg);
    }
    hours.push({ hour: e.hour, pm });
  }
  hours.sort((a, b) => Date.parse(a.hour) - Date.parse(b.hour));
  return { hours };
}

// Module-level cache keyed by full URL (the API key is sent in a header, never in the URL).
const cache = new Map<string, { at: number; value: Promise<unknown> }>();

export function __resetCacheForTests(): void {
  cache.clear();
}

export class Aai {
  constructor(
    private base: string,
    private key: string,
    private fetchFn: typeof fetch,
    private now: () => number,
  ) {}

  map(city: CityCode): Promise<MapSnapshot> {
    return this.get(`${this.base}/measurements/map?city=${city}`, parseMap);
  }

  history(city: CityCode): Promise<History> {
    return this.get(`${this.base}/measurements/history?hours=6&city=${city}`, parseHistory);
  }

  private get<T>(url: string, parse: (j: unknown) => T): Promise<T> {
    const hit = cache.get(url);
    if (hit && this.now() - hit.at < TTL_MS) return hit.value as Promise<T>;
    // Cache the in-flight promise so concurrent cold requests share one fetch;
    // drop it on failure so errors are never cached.
    const entry = { at: this.now(), value: this.fetchParsed(url, parse) as Promise<unknown> };
    cache.set(url, entry);
    entry.value.catch(() => {
      if (cache.get(url) === entry) cache.delete(url);
    });
    return entry.value as Promise<T>;
  }

  private async fetchParsed<T>(url: string, parse: (j: unknown) => T): Promise<T> {
    try {
      const res = await this.fetchFn(url, {
        headers: { "x-api-key": this.key },
        signal: AbortSignal.timeout(TIMEOUT_MS),
      });
      if (!res.ok) throw new UpstreamError(`upstream status ${res.status}`);
      return parse(await res.json());
    } catch (e) {
      if (e instanceof UpstreamError) throw e;
      throw new UpstreamError(`upstream request failed: ${e instanceof Error ? e.name : "error"}`);
    }
  }
}
