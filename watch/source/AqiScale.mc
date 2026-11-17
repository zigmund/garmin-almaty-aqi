import Toybox.Lang;

// Geometry of the glance's AQI scale: 6 equal segments (one per EPA category) with gaps,
// and the marker position for a value, interpolated inside its category's segment.
(:glance)
module AqiScale {
    const SEGMENTS = 6;

    // [x, width] of segment i (0..5) in a scale `width` px wide with `gap` px between segments.
    function segment(i as Number, width as Number, gap as Number) as Array<Number> {
        var segW = (width - (SEGMENTS - 1) * gap) / SEGMENTS;
        return [i * (segW + gap), segW];
    }

    // X of the marker for an AQI value; values outside 0..500 are clamped to the ends.
    function markerX(aqi as Number, width as Number, gap as Number) as Number {
        if (aqi < 0) { aqi = 0; }
        if (aqi > 500) { aqi = 500; }
        var cat = Aqi.category(aqi);
        var lows = [0, 51, 101, 151, 201, 301];
        var highs = [50, 100, 150, 200, 300, 500];
        var lo = lows[cat];
        var span = highs[cat] - lo;
        var seg = segment(cat, width, gap);
        // Rounded (aqi - lo) / span of the segment's last pixel offset.
        var off = ((aqi - lo) * (seg[1] - 1) * 2 + span) / (2 * span);
        return seg[0] + off;
    }
}
