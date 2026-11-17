import Toybox.Background;
import Toybox.Communications;
import Toybox.Lang;
import Toybox.System;

// Fetches /v1/glance every 5 minutes and hands the result to AirMonApp.onBackgroundData.
// The position comes from the weather observation (WxPos); none is sent when it is unavailable.
(:background)
class BgService extends System.ServiceDelegate {
    function initialize() {
        ServiceDelegate.initialize();
    }

    function onTemporalEvent() as Void {
        Communications.makeWebRequest(
            Api.glanceUrl(WxPos.read(), Store.place()),
            null,
            {:method => Communications.HTTP_REQUEST_METHOD_GET, :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            method(:onResp)
        );
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        Background.exit(code == 200 ? data : {"err" => code});
    }
}
