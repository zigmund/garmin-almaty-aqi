import Toybox.Background;
import Toybox.Communications;
import Toybox.Lang;
import Toybox.Position;
import Toybox.System;
import Toybox.Time;

// Fetches /v1/glance every 5 minutes and hands the result to AirMonApp.onBackgroundData.
//
// Position in the background (spec hypothesis): Position.getInfo() may be unavailable to a
// background process on CIQ 3.4. The read is therefore wrapped in try/catch; on any failure
// Fix.pick falls back to the fix stored by the widget (Store.fix()).
// Observed in the simulator: see task report (not verified automatically).
(:background)
class BgService extends System.ServiceDelegate {
    function initialize() {
        ServiceDelegate.initialize();
    }

    function onTemporalEvent() as Void {
        var now = Time.now().value();
        var fix = Fix.pick(readPosition(), Store.fix(), now);
        Communications.makeWebRequest(
            Api.glanceUrl(fix, Store.lastCity()),
            null,
            {:method => Communications.HTTP_REQUEST_METHOD_GET, :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            method(:onResp)
        );
    }

    // [lat, lon, timeSec] from the current position, or null if unavailable.
    function readPosition() as Array? {
        try {
            var info = Position.getInfo();
            if (info == null || info.position == null || info.when == null) { return null; }
            if (info.accuracy < Position.QUALITY_LAST_KNOWN) { return null; }
            var deg = info.position.toDegrees();
            return [deg[0], deg[1], info.when.value()];
        } catch (ex) {
            return null;
        }
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        Background.exit(code == 200 ? data : {"err" => code});
    }
}
