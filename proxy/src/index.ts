// Cloudflare Worker entry: routes /v1/glance and /v1/heatmap over the AAI client.

import { Aai, UpstreamError, type History, type MapSnapshot } from "./aai";
import { candidateCities, isCity, type CityCode } from "./cities";
import { buildGlance, resolve } from "./glance";
import { buildHeatmap } from "./heatmap";

export interface Env {
  AAI_BASE: string;
  AAI_API_KEY: string;
}

type Pos = { lat: number; lon: number };

const reply = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

const err = (code: string, status: number) => reply({ err: code }, status);

// Returns null for "no coordinates", undefined for invalid input, else the position.
function parsePos(q: URLSearchParams): Pos | null | undefined {
  const rawLat = q.get("lat");
  const rawLon = q.get("lon");
  const check = (raw: string | null, max: number): number | null | undefined => {
    if (raw === null) return null;
    const v = raw.trim() === "" ? NaN : Number(raw);
    return Number.isFinite(v) && Math.abs(v) <= max ? v : undefined;
  };
  const lat = check(rawLat, 90);
  const lon = check(rawLon, 180);
  if (lat === undefined || lon === undefined) return undefined;
  if (lat === null || lon === null) return null;
  return { lat, lon };
}

export async function handle(req: Request, aai: Aai): Promise<Response> {
  try {
    if (req.method !== "GET") return err("method", 405);
    const url = new URL(req.url);
    const isGlance = url.pathname === "/v1/glance";
    if (!isGlance && url.pathname !== "/v1/heatmap") return err("not_found", 404);

    const pos = parsePos(url.searchParams);
    if (pos === undefined) return err("bad_request", 400);
    const cityParam = url.searchParams.get("city") ?? "";
    const lastCity: CityCode = isCity(cityParam) ? cityParam : "almaty";

    const cities = pos ? candidateCities(pos.lat, pos.lon) : [lastCity];
    const loaded = await Promise.all(cities.map(async (c) => [c, await aai.map(c)] as const));
    const maps: Partial<Record<CityCode, MapSnapshot>> = Object.fromEntries(loaded);

    const r = resolve(maps, pos, lastCity);
    if (isGlance) {
      let hist: History | null = null;
      if (r.mode !== "out") hist = await aai.history(r.city).catch(() => null);
      return reply(buildGlance(r, r.mode === "out" ? null : (maps[r.city] ?? null), hist, aai.nowMs()));
    }
    if (r.mode === "out") return reply({ m: "out" });
    const map = maps[r.city];
    if (!map) return err("upstream", 502);
    return reply(buildHeatmap(r, map, pos, 24, aai.nowMs()));
  } catch (e) {
    // Never expose or log error details: they could carry upstream URLs or headers.
    // Upstream failures are already logged (status / class name) in aai.ts.
    if (!(e instanceof UpstreamError)) console.error(`handler failed: ${e instanceof Error ? e.name : "error"}`);
    return err("upstream", 502);
  }
}

export default {
  fetch(req: Request, env: Env): Promise<Response> {
    if (!env.AAI_API_KEY) return Promise.resolve(err("config", 500));
    return handle(req, new Aai(env.AAI_BASE, env.AAI_API_KEY, (i, init) => fetch(i, init), Date.now));
  },
};
