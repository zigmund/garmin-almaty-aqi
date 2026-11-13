import Toybox.Lang;

// Shape and freshness of the stored /v1/glance payload. "rx" is the receive time (Unix s).
(:glance, :background)
module GlanceData {
    enum { NO_DATA, OUT, NEAR, CITY }

    const STALE_SEC = 1800;
    const NEVER_MIN = 9999;

    function state(d as Dictionary?) as Number {
        if (!(d instanceof Dictionary)) { return NO_DATA; }
        var m = d["m"];
        if (!(m instanceof String)) { return NO_DATA; }
        if (m.equals("out")) { return OUT; }
        var s = m.equals("near") ? NEAR : (m.equals("city") ? CITY : NO_DATA);
        if (s != NO_DATA && !(d["aqi"] instanceof Number && d["h"] instanceof Array)) { return NO_DATA; }
        return s;
    }

    function ageMin(d as Dictionary, nowSec as Number) as Number {
        var rx = d["rx"];
        if (!(rx instanceof Number)) { return NEVER_MIN; }
        var age = nowSec - rx;
        return age < 0 ? 0 : age / 60;
    }

    function isStale(d as Dictionary, nowSec as Number) as Boolean {
        var rx = d["rx"];
        if (!(rx instanceof Number)) { return true; }
        return nowSec - rx > STALE_SEC;
    }
}
