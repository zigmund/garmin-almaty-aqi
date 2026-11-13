import Toybox.Lang;
import Toybox.Math;

// Bar heights for the hourly PM2.5 AQI history.
(:glance)
module Bars {
    const MIN_SCALE = 150;

    // Pixel height per value, scaled to max(150, largest value) so a clean hour is not a full bar.
    // Non-zero values get at least 1 px; null (no data) becomes -1.
    function heights(h as Array, maxPx as Number) as Array<Number> {
        var scale = MIN_SCALE;
        for (var i = 0; i < h.size(); i++) {
            var v = h[i];
            if (v instanceof Number && v > scale) { scale = v; }
        }
        var out = new Array<Number>[h.size()];
        for (var i = 0; i < h.size(); i++) {
            var v = h[i];
            if (!(v instanceof Number)) {
                out[i] = -1;
            } else {
                var px = Math.round(v.toFloat() / scale * maxPx).toNumber();
                out[i] = (v > 0 && px < 1) ? 1 : px;
            }
        }
        return out;
    }
}
