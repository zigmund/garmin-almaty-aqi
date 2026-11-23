import Toybox.Background;
import Toybox.Communications;
import Toybox.Lang;
import Toybox.System;

// Fetches /v1/glance every 5 minutes and hands the result to AlmatyAqiApp.onBackgroundData.
// With a manual city only the city is sent; otherwise the weather-observation position (WxPos,
// none when unavailable) plus the last auto city. The payload gets "hp": the request had a position.
(:background)
class BgService extends System.ServiceDelegate {
    private var _hp as Boolean = false;

    function initialize() {
        ServiceDelegate.initialize();
    }

    function onTemporalEvent() as Void {
        var p = Place.request();
        _hp = p[0] != null;
        Communications.makeWebRequest(
            Api.glanceUrl(p[0], p[1]),
            null,
            {:method => Communications.HTTP_REQUEST_METHOD_GET, :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            method(:onResp)
        );
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        if (code == 200 && data instanceof Dictionary) { data["hp"] = _hp; }
        Background.exit(code == 200 ? data : {"err" => code});
    }
}
