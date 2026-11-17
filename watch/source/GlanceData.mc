import Toybox.Lang;

// Shape and freshness of the stored /v1/glance payload. "rx" is the receive time (Unix s); "dt" the data time = rx - "a" (age at response).
(:glance, :background)
module GlanceData {
    enum { NO_DATA, OUT, CITY, NOPOS }

    const STALE_SEC = 1800;
    const NEVER_MIN = 9999;

    function state(d as Dictionary?) as Number {
        if (!(d instanceof Dictionary)) { return NO_DATA; }
        var m = d["m"];
        if (!(m instanceof String)) { return NO_DATA; }
        if (m.equals("out")) { return OUT; }
        if (m.equals("nopos")) { return NOPOS; }
        if (m.equals("city") && d["aqi"] instanceof Number) { return CITY; }
        return NO_DATA;
    }

    // Data time: "dt" if stored, else the receive time (payloads stored by older builds).
    function stamp(d as Dictionary) as Number? {
        var t = d["dt"];
        if (!(t instanceof Number)) { t = d["rx"]; }
        return t instanceof Number ? t : null;
    }

    // Unix time of the data: receive time minus the server's age "a"; a bad "a" means rx.
    function dataTime(a as Object?, rxSec as Number) as Number {
        return (a instanceof Number && a >= 0) ? rxSec - a : rxSec;
    }

    function ageMin(d as Dictionary, nowSec as Number) as Number {
        var t = stamp(d);
        if (t == null) { return NEVER_MIN; }
        var age = nowSec - t;
        return age < 0 ? 0 : age / 60;
    }

    // Age for display: [value, inHours]. Whole hours (floor) from 60 min; "?" when never received.
    function ageParts(mins as Number) as [String, Boolean] {
        if (mins >= NEVER_MIN) { return ["?", false]; }
        if (mins < 60) { return [mins.toString(), false]; }
        return [(mins / 60).toString(), true];
    }

    function isStale(d as Dictionary, nowSec as Number) as Boolean {
        var t = stamp(d);
        if (t == null) { return true; }
        return nowSec - t > STALE_SEC;
    }

    // Dictionary to store for a background result, or null to keep the old one (errors, junk).
    function onReceive(data as Object?, nowSec as Number) as Dictionary? {
        if (!(data instanceof Dictionary) || !(data["m"] instanceof String)) { return null; }
        data["rx"] = nowSec;
        data["dt"] = dataTime(data["a"], nowSec);
        return data;
    }

    // Place to remember for the next request: "out" when out of zone, the city code when the
    // server resolved one; null otherwise (nopos, junk).
    function placeToStore(d as Dictionary) as String? {
        var m = d["m"];
        var c = d["c"];
        if (!(m instanceof String)) { return null; }
        if (m.equals("out")) { return "out"; }
        if (m.equals("city") && c instanceof String) { return c; }
        return null;
    }
}
