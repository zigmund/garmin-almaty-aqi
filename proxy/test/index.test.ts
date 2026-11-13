import { beforeEach, describe, expect, it } from "vitest";
import mapFx from "./fixtures/map-almaty.json";
import histFx from "./fixtures/history-almaty.json";
import { Aai, __resetCacheForTests } from "../src/aai";
import { __resetHeatmapCacheForTests } from "../src/heatmap";
import { handle } from "../src/index";

const KEY = "test-key";
const body = (r: Response): Promise<any> => r.json();
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

interface Opts {
  mapStatus?: number;
  histStatus?: number;
}
function setup(o: Opts = {}) {
  const calls: string[] = [];
  const fetchFn = (async (input: RequestInfo | URL) => {
    const url = String(input);
    calls.push(url);
    if (url.includes("/measurements/map")) {
      return url.includes("city=almaty") && !o.mapStatus ? json(mapFx) : new Response("{}", { status: o.mapStatus ?? 404 });
    }
    if (url.includes("/measurements/history")) {
      return o.histStatus ? new Response("{}", { status: o.histStatus }) : json(histFx);
    }
    return new Response("{}", { status: 404 });
  }) as typeof fetch;
  const aai = new Aai("https://up.test/v1", KEY, fetchFn, Date.now);
  const call = (path: string, method = "GET") => handle(new Request(`https://w.test${path}`, { method }), aai);
  return { calls, call };
}

beforeEach(() => {
  __resetCacheForTests();
  __resetHeatmapCacheForTests();
});

describe("/v1/glance", () => {
  it("near with coordinates", async () => {
    const { call } = setup();
    const res = await call("/v1/glance?lat=43.25&lon=76.9");
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type")).toBe("application/json");
    const b = await body(res);
    expect(b.m).toBe("near");
    expect(b.h.length).toBe(6);
  });
  it("a is a non-negative integer age in seconds", async () => {
    const { call } = setup();
    const b = await body(await call("/v1/glance?lat=43.25&lon=76.9"));
    expect(Number.isInteger(b.a)).toBe(true);
    expect(b.a).toBeGreaterThanOrEqual(0);
  });
  it("city without coordinates", async () => {
    const { call, calls } = setup();
    const b = await body(await call("/v1/glance"));
    expect(b.m).toBe("city");
    expect(b.c).toBe("almaty");
    expect(calls.filter((u) => u.includes("map")).length).toBe(1);
  });
  it("unknown city falls back to almaty", async () => {
    const { call } = setup();
    const b = await body(await call("/v1/glance?city=paris"));
    expect(b.c).toBe("almaty");
  });
  it("only one coordinate is treated as none", async () => {
    const { call } = setup();
    const b = await body(await call("/v1/glance?lat=43.25"));
    expect(b.m).toBe("city");
  });
  it("out of coverage makes no upstream call", async () => {
    const { call, calls } = setup();
    const res = await call("/v1/glance?lat=51.13&lon=71.43");
    expect(await res.json()).toEqual({ m: "out" });
    expect(calls.length).toBe(0);
  });
  it.each(["lat=abc&lon=76.9", "lat=95&lon=76.9", "lat=43&lon=181", "lat=43&lon=", "lat=abc"])("400 on %s", async (q) => {
    const { call } = setup();
    const res = await call(`/v1/glance?${q}`);
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ err: "bad_request" });
  });
  it("502 when upstream map fails", async () => {
    const { call } = setup({ mapStatus: 500 });
    const res = await call("/v1/glance?lat=43.25&lon=76.9");
    expect(res.status).toBe(502);
    expect(await res.json()).toEqual({ err: "upstream" });
  });
  it("degrades when history fails", async () => {
    const { call } = setup({ histStatus: 500 });
    const res = await call("/v1/glance?lat=43.25&lon=76.9");
    expect(res.status).toBe(200);
    expect((await body(res)).m).toBe("near");
  });
});

describe("/v1/heatmap", () => {
  it("returns grid", async () => {
    const { call, calls } = setup();
    const res = await call("/v1/heatmap?lat=43.25&lon=76.9");
    expect(res.status).toBe(200);
    const b = await body(res);
    expect(b.g.length).toBe(Math.ceil(576 / 3) * 4);
    expect(calls.some((u) => u.includes("history"))).toBe(false);
    expect(Number.isInteger(b.a)).toBe(true);
    expect(b.a).toBeGreaterThanOrEqual(0);
  });
});

describe("routing", () => {
  it("404 on unknown path", async () => {
    const { call } = setup();
    const res = await call("/v1/nope");
    expect(res.status).toBe(404);
    expect(await res.json()).toEqual({ err: "not_found" });
  });
  it("405 on non-GET", async () => {
    const { call } = setup();
    const res = await call("/v1/glance", "POST");
    expect(res.status).toBe(405);
    expect(await res.json()).toEqual({ err: "method" });
  });
  it("never leaks the key", async () => {
    const { call } = setup({ mapStatus: 500 });
    for (const p of ["/v1/glance?lat=43.25&lon=76.9", "/v1/heatmap", "/x", "/v1/glance?lat=abc"]) {
      const res = await call(p);
      expect(res.headers.get("content-type")).toBe("application/json");
      expect(await res.text()).not.toContain(KEY);
    }
  });
});
