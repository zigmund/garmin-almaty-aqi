import Toybox.Lang;
import Toybox.Test;

function endsWith(s as String, suffix as String) as Boolean {
    var n = s.length();
    var k = suffix.length();
    return n >= k && s.substring(n - k, n).equals(suffix);
}

(:test)
function apiUrls(logger as Logger) as Boolean {
    Test.assertEqual(Api.glanceUrl(null, null), BuildInfo.PROXY_URL + "/v1/glance");
    Test.assertEqual(Api.glanceUrl([43.2389, 76.8897], "almaty"), BuildInfo.PROXY_URL + "/v1/glance?city=almaty&lat=43.239&lon=76.890");
    Test.assertEqual(Api.heatmapUrl(null, "out"), BuildInfo.PROXY_URL + "/v1/heatmap?city=out");
    return true;
}
