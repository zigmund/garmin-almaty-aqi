import Toybox.Lang;

// US AQI categories (EPA bands) and their MIP-friendly colors.
(:glance, :background)
module Aqi {
    const GRAY = 0xAAAAAA;

    // Category index 0..5 for an AQI value.
    function category(aqi as Number) as Number {
        if (aqi <= 50) { return 0; }
        if (aqi <= 100) { return 1; }
        if (aqi <= 150) { return 2; }
        if (aqi <= 200) { return 3; }
        if (aqi <= 300) { return 4; }
        return 5;
    }

    function color(cat as Number) as Number {
        var colors = [0x00AA00, 0xFFFF00, 0xFFAA00, 0xFF0000, 0xAA00AA, 0x550000];
        return colors[cat];
    }
}
