# airmon-proxy

Cloudflare Worker that sits between the Garmin watch app and the
[Almaty Air Initiative (AAI)](https://air.org.kz) API. It hides the API key,
caches upstream data for 5 minutes and returns compact JSON for the watch.

Air quality data: AAI, licensed under CC BY 4.0. Code: MIT.

## Local setup

```sh
npm i
echo 'AAI_API_KEY=<your key>' > .dev.vars   # gitignored, never commit it
npm run dev                                  # http://localhost:8787
npm test && npm run typecheck
```

## Deploy

```sh
npx wrangler login
npx wrangler secret put AAI_API_KEY
npm run deploy
```

`AAI_BASE` is set in `wrangler.toml`; the key only lives in `.dev.vars` and in
the Worker secret.

## API

All responses are `application/json`. Errors: `{"err":"bad_request"}` (400,
invalid `lat`/`lon`), `{"err":"not_found"}` (404), `{"err":"method"}` (405),
`{"err":"upstream"}` (502), `{"err":"config"}` (500, missing key).

Query parameters (both endpoints):

- `lat`, `lon` - optional position. If only one is given, it is ignored.
- `city` - `almaty` (default) or `karaganda`; used when there is no position.

### `GET /v1/glance`

- `{"m":"out"}` - position is outside coverage.
- `{"m":"near"|"city","c":<city>,"aqi":<int>,"pm":<PM2.5, 1 decimal>,"h":[...],"t":<time>,"a":<age s>,"st":<stations>,"fb":<bool>}`
  - `near`: IDW of the nearest stations; `city`: city median.
  - `h`: 6 hourly AQI values (null where no data), `st`: stations used,
    `fb`: upstream served fallback data. `a`: age of the data in whole seconds at response
    time (>= 0, from `t`; omitted if `t` is unusable); the watch derives the data time as
    receive time minus `a`.

### `GET /v1/heatmap`

- `{"m":"out"}` or
  `{"m":"near"|"city","c","bbox":[w,s,e,n],"w":24,"h":24,"g":"...","s":[[lat,lon,cat],...],"me":[x,y],"t","a"}`
- `g`: base64 of `w*h` bytes, row-major from the north-west corner; each byte
  is an AQI category 0-5, or 255 for a cell with no data within 3 km.
- `s`: station dots with their category; `me`: your position as [x, y] fractions (0-1) of the bbox from its north-west corner, present only when inside the bbox.
