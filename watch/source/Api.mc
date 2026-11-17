import Toybox.Lang;

// Proxy request URLs. The position is sent rounded to 3 decimals (~100 m).
// Parameters (all optional): city=<place>, lat, lon.
(:glance, :background)
module Api {
    function glanceUrl(pos as Array?, place as String?) as String {
        return url("/v1/glance", pos, place);
    }

    function heatmapUrl(pos as Array?, place as String?) as String {
        return url("/v1/heatmap", pos, place);
    }

    function url(path as String, pos as Array?, place as String?) as String {
        var u = BuildInfo.PROXY_URL + path;
        var sep = "?";
        if (place != null) {
            u += sep + "city=" + place;
            sep = "&";
        }
        if (pos != null) {
            u += sep + "lat=" + (pos[0] as Number or Float or Double).toFloat().format("%.3f") + "&lon=" + (pos[1] as Number or Float or Double).toFloat().format("%.3f");
        }
        return u;
    }
}
