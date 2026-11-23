import Toybox.Lang;

// City choice: an explicit "manual" city wins; otherwise the position decides and the last city
// the server resolved ("lastAuto") is only a hint for the next request.
(:glance, :background)
module Place {
    // The city code if v is one we know, else null.
    function cityOrNull(v as Object?) as String? {
        if (v instanceof String && (v.equals("almaty") || v.equals("karaganda"))) { return v; }
        return null;
    }

    // [pos, city] for a request: a manual city is sent alone, otherwise the position plus the last auto city.
    function params(manual as String?, wx as Array?, lastAuto as String?) as Array {
        if (manual != null) { return [null, manual]; }
        return [wx, lastAuto];
    }

    function request() as Array {
        var manual = Store.manual();
        return params(manual, manual == null ? WxPos.read() : null, Store.lastAuto());
    }

    // City to remember as lastAuto from a response, or null. Only a response to a request that
    // carried a position (hadPos) is auto-detection; a bare "city=<lastAuto>" request just echoes.
    function autoToStore(m as String?, c as String?, manual as String?, hadPos as Boolean) as String? {
        if (hadPos && manual == null && m != null && m.equals("city") && cityOrNull(c) != null) { return c; }
        return null;
    }

    // Whether to ask the user to pick a city: no manual choice, not asked yet, and auto failed.
    function needsAsk(m as String?, manual as String?, asked as Boolean) as Boolean {
        return manual == null && !asked && m != null && (m.equals("nopos") || m.equals("out"));
    }

    function onResult(m as String?, c as String?, hadPos as Boolean) as Void {
        var a = autoToStore(m, c, Store.manual(), hadPos);
        if (a != null) {
            Store.setLastAuto(a);
            if (Store.asked()) { Store.setAsked(false); }
        }
    }

    // Stores a glance result and remembers its city; returns what was stored (null for errors).
    // "hp" in the payload: the request carried a position (set by the requester).
    function applyGlance(data as Object?, nowSec as Number) as Dictionary? {
        var d = GlanceData.onReceive(data, nowSec);
        if (d != null) {
            Store.setGlance(d);
            var m = d["m"];
            var c = d["c"];
            onResult(m instanceof String ? m : null, c instanceof String ? c : null, d["hp"] == true);
        }
        return d;
    }

    // Manual city, else the last auto one (the map frame before the first response).
    function city() as String? {
        var m = Store.manual();
        return m != null ? m : Store.lastAuto();
    }
}
