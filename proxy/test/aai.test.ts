import { beforeEach, describe, expect, it } from "vitest";
import mapFx from "./fixtures/map-almaty.json";
import histFx from "./fixtures/history-almaty.json";
import { Aai, UpstreamError, __resetCacheForTests, parseHistory, parseMap } from "../src/aai";
import { candidateCities, isCity } from "../src/cities";

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

describe("parseMap", () => {
  it("skips stations without coords or pm", () => {
    const snap = parseMap(mapFx);
    expect(snap.stations.length).toBe(mapFx.data.length - 3);
    for (const s of snap.stations) {
      expect(Number.isFinite(s.lat)).toBe(true);
      expect(Number.isFinite(s.lon)).toBe(true);
      expect(Number.isFinite(s.pm)).toBe(true);
    }
    expect(snap.stations[0]).toEqual({
      id: mapFx.data[0].location_id,
      lat: mapFx.data[0].latitude,
      lon: mapFx.data[0].longitude,
      pm: mapFx.data[0].pm25_avg,
    });
  });
  it("reads windowEnd and fallback from meta", () => {
    const snap = parseMap(mapFx);
    expect(snap.windowEnd).toBe(mapFx.meta.window_end);
    expect(snap.fallback).toBe(false);
    expect(parseMap({ data: [], meta: { window_end: "x", is_fallback: true } }).fallback).toBe(true);
  });
  it("throws UpstreamError on bad shape", () => {
    expect(() => parseMap({})).toThrow(UpstreamError);
    expect(() => parseMap(null)).toThrow(UpstreamError);
  });
});

describe("parseHistory", () => {
  it("returns hours ascending with pm by id", () => {
    const h = parseHistory(histFx);
    expect(h.hours.length).toBe(7);
    const times = h.hours.map((x) => Date.parse(x.hour));
    expect([...times].sort((a, b) => a - b)).toEqual(times);
    const loc = histFx.data[0].locations[0];
    expect(h.hours[0].pm.get(loc.location_id)).toBe(loc.pm25_avg);
  });
  it("sorts a reversed response", () => {
    const rev = { ...histFx, data: [...histFx.data].reverse() };
    const h = parseHistory(rev);
    expect(h.hours[0].hour).toBe(histFx.data[0].hour);
  });
  it("throws UpstreamError on bad shape", () => {
    expect(() => parseHistory({ nope: 1 })).toThrow(UpstreamError);
  });
});

describe("Aai", () => {
  beforeEach(() => __resetCacheForTests());

  function setup(handler?: (url: string, init?: RequestInit) => Response | Promise<Response>) {
    const calls: { url: string; init?: RequestInit }[] = [];
    const fetchFn = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      calls.push({ url, init });
      return handler ? handler(url, init) : json(url.includes("/history") ? histFx : mapFx);
    }) as typeof fetch;
    let t = 1_000_000;
    const aai = new Aai("https://aai.test/v1", "test-key", fetchFn, () => t);
    return { aai, calls, advance: (s: number) => (t += s * 1000) };
  }

  it("sends x-api-key and city", async () => {
    const { aai, calls } = setup();
    await aai.map("almaty");
    expect(calls[0].url).toBe("https://aai.test/v1/measurements/map?city=almaty");
    expect(new Headers(calls[0].init?.headers).get("x-api-key")).toBe("test-key");
    expect(calls[0].url).not.toContain("test-key");
  });
  it("history requests hours=6", async () => {
    const { aai, calls } = setup();
    const h = await aai.history("almaty");
    expect(calls[0].url).toBe("https://aai.test/v1/measurements/history?hours=6&city=almaty");
    expect(h.hours.length).toBe(7);
  });
  it("caches within 300 s, refetches after", async () => {
    const { aai, calls, advance } = setup();
    await aai.map("almaty");
    advance(299);
    await aai.map("almaty");
    expect(calls.length).toBe(1);
    advance(2);
    await aai.map("almaty");
    expect(calls.length).toBe(2);
  });
  it("shares cache across instances, separate per city", async () => {
    const a = setup();
    await a.aai.map("almaty");
    await new Aai("https://aai.test/v1", "test-key", (async () => {
      throw new Error("should not fetch");
    }) as typeof fetch, () => 1_000_000).map("almaty");
    await a.aai.map("karaganda");
    expect(a.calls.length).toBe(2);
  });
  it("dedupes concurrent cold requests", async () => {
    const { aai, calls } = setup();
    const [a, b] = await Promise.all([aai.map("almaty"), aai.map("almaty")]);
    expect(calls.length).toBe(1);
    expect(a).toBe(b);
  });
  it("throws UpstreamError on 500 and does not cache it", async () => {
    const { aai, calls } = setup(() => new Response("boom", { status: 500 }));
    await expect(aai.map("almaty")).rejects.toThrow(UpstreamError);
    await expect(aai.map("almaty")).rejects.toThrow(UpstreamError);
    expect(calls.length).toBe(2);
  });
  it("throws UpstreamError on network error / timeout and passes a signal", async () => {
    const { aai, calls } = setup(() => {
      throw new DOMException("timed out", "TimeoutError");
    });
    await expect(aai.map("almaty")).rejects.toThrow(UpstreamError);
    expect(calls[0].init?.signal).toBeInstanceOf(AbortSignal);
  });
  it("throws UpstreamError on unparsable JSON", async () => {
    const { aai } = setup(() => new Response("not json", { status: 200 }));
    await expect(aai.map("almaty")).rejects.toThrow(UpstreamError);
    const s2 = setup(() => json({ hello: 1 }));
    await expect(s2.aai.history("almaty")).rejects.toThrow(UpstreamError);
  });
});

describe("cities", () => {
  it("candidateCities", () => {
    expect(candidateCities(43.25, 76.9)).toEqual(["almaty"]);
    expect(candidateCities(51.1, 71.4)).toEqual([]);
    expect(candidateCities(49.8, 73.1)).toEqual(["karaganda"]);
  });
  it("isCity", () => {
    expect(isCity("almaty")).toBe(true);
    expect(isCity("astana")).toBe(false);
    expect(isCity("toString")).toBe(false);
  });
});
