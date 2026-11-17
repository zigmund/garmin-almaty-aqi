import Toybox.Lang;

// Pure helpers for the MapView screen: /v1/heatmap payload -> map markers in degrees.
// Foreground only (the full widget view).
module MapModel {
    const GRAY = 6;  // marker index for "no data" / unknown category

    // Marker bitmap index for an AQI category 0..5; anything else is gray.
    function markerIndex(cat) as Number {
        if (cat instanceof Number && cat >= 0 && cat <= 5) { return cat; }
        return GRAY;
    }

    // [w, s, e, n] in degrees, same as proxy/src/cities.ts; unknown codes fall back to Almaty.
    // Used only to frame the map before the first response arrives.
    function cityBbox(code as String) as Array<Float> {
        if (code.equals("karaganda")) { return [72.9, 49.7, 73.3, 49.95]; }
        return [76.6, 43.14, 77.26, 43.5];
    }

    // Fraction of the bbox (x right, y down from the top-left) -> [lat, lon].
    function fracToLatLon(bbox as Array, fx as Float, fy as Float) as Array<Float> {
        var w = bbox[0].toFloat();
        var s = bbox[1].toFloat();
        var e = bbox[2].toFloat();
        var n = bbox[3].toFloat();
        return [n - fy * (n - s), w + fx * (e - w)];
    }

    // {"m" => "out"}, or {"m" => "map", "bbox" => [w, s, e, n], "st" => [[lat, lon, markerIndex], ...],
    // "me" => [lat, lon] or null}; null for anything malformed.
    function parse(data as Dictionary) as Dictionary? {
        var m = data["m"];
        if (!(m instanceof String)) { return null; }
        if (m.equals("out")) { return {"m" => "out"}; }
        if (!m.equals("near") && !m.equals("city")) { return null; }
        var bb = data["bbox"];
        if (!(bb instanceof Array) || bb.size() != 4) { return null; }
        var bbox = new Array<Float>[4];
        for (var i = 0; i < 4; i++) {
            if (!isNum(bb[i])) { return null; }
            bbox[i] = bb[i].toFloat();
        }

        var st = [];
        var s = data["s"];
        if (s instanceof Array) {
            for (var i = 0; i < s.size(); i++) {
                var p = s[i];
                if (!(p instanceof Array) || p.size() < 3 || !isNum(p[0]) || !isNum(p[1]) || !(p[2] instanceof Number)) { continue; }
                var lat = p[0].toFloat();
                var lon = p[1].toFloat();
                if (lon < bbox[0] || lon > bbox[2] || lat < bbox[1] || lat > bbox[3]) { continue; }
                st.add([lat, lon, markerIndex(p[2])]);
            }
        }

        var you = null;
        var mv = data["me"];
        if (mv instanceof Array && mv.size() == 2 && isNum(mv[0]) && isNum(mv[1])) {
            var fx = mv[0].toFloat();
            var fy = mv[1].toFloat();
            if (fx >= 0.0 && fx <= 1.0 && fy >= 0.0 && fy <= 1.0) { you = fracToLatLon(bbox, fx, fy); }
        }
        return {"m" => "map", "bbox" => bbox, "st" => st, "me" => you};
    }

    function isNum(v) as Boolean {
        return v instanceof Number || v instanceof Float || v instanceof Double || v instanceof Long;
    }
}
