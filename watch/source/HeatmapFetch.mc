import Toybox.Communications;
import Toybox.Lang;
import Toybox.Position;
import Toybox.Timer;

// One /v1/heatmap request per start(): a fresh GPS fix if it comes within POS_TIMEOUT_MS,
// else the weather position (or none). Shared by HeatmapView and MapAirView. Foreground only.
class HeatmapFetch {
    private const POS_TIMEOUT_MS = 10000;

    private var _onResp as Method(code as Number, data as Dictionary or String or Null) as Void;
    private var _requested as Boolean = false;
    private var _pos as Array? = null;
    private var _timer as Timer.Timer? = null;

    function initialize(onResp as Method(code as Number, data as Dictionary or String or Null) as Void) {
        _onResp = onResp;
    }

    function start() as Void {
        stopTimer();
        _requested = false;
        _timer = new Timer.Timer();
        (_timer as Timer.Timer).start(method(:onTimeout), POS_TIMEOUT_MS, false);
        Position.enableLocationEvents(Position.LOCATION_ONE_SHOT, method(:onPos));
    }

    function stop() as Void {
        stopTimer();
        Position.enableLocationEvents(Position.LOCATION_DISABLE, method(:onPos));
    }

    function onPos(info as Position.Info) as Void {
        if (_requested || info.position == null) { return; }
        var deg = (info.position as Position.Location).toDegrees();
        request([deg[0], deg[1]]);
    }

    function onTimeout() as Void {
        if (_requested) { return; }
        request(WxPos.read());
    }

    // [lat, lon] sent in the last request (GPS or weather), or null.
    function pos() as Array? {
        return _pos;
    }

    private function stopTimer() as Void {
        if (_timer != null) {
            (_timer as Timer.Timer).stop();
            _timer = null;
        }
    }

    // At most one request per start(): onPos and the timeout race for it.
    private function request(pos as Array?) as Void {
        if (_requested) { return; }
        _requested = true;
        _pos = pos;
        stopTimer();
        Communications.makeWebRequest(
            Api.heatmapUrl(pos, Store.place()),
            null,
            {:method => Communications.HTTP_REQUEST_METHOD_GET, :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            _onResp
        );
    }
}
