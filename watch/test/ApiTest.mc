import Toybox.Lang;
import Toybox.Test;

function endsWith(s as String, suffix as String) as Boolean {
    var n = s.length();
    var k = suffix.length();
    return n >= k && s.substring(n - k, n).equals(suffix);
}

(:test)
function apiGlanceUrlWithoutFix(logger as Logger) as Boolean {
    Test.assert(endsWith(Api.glanceUrl(null, "karaganda"), "/v1/glance?city=karaganda"));
    return true;
}

(:test)
function apiGlanceUrlWithFix(logger as Logger) as Boolean {
    var u = Api.glanceUrl([43.23891, 76.88972, 0], "almaty");
    Test.assert(u.find("/v1/glance?city=almaty&lat=43.239&lon=76.890") != null);
    return true;
}

(:test)
function apiHeatmapUrl(logger as Logger) as Boolean {
    Test.assert(endsWith(Api.heatmapUrl(null, "almaty"), "/v1/heatmap?city=almaty"));
    return true;
}
