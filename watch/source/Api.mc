import Toybox.Lang;

// Proxy request URLs.
(:glance, :background)
module Api {
    function glanceUrl(fix as Array?, city as String) as String {
        return url("/v1/glance?city=", fix, city);
    }

    function heatmapUrl(fix as Array?, city as String) as String {
        return url("/v1/heatmap?city=", fix, city);
    }

    function url(path as String, fix as Array?, city as String) as String {
        var u = BuildInfo.PROXY_URL + path + city;
        if (fix != null) {
            u += "&lat=" + (fix[0] as Number or Float or Double).toFloat().format("%.4f") + "&lon=" + (fix[1] as Number or Float or Double).toFloat().format("%.4f");
        }
        return u;
    }
}
