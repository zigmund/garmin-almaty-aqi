import Toybox.Communications;
import Toybox.Lang;
import Toybox.Position;
import Toybox.WatchUi;

typedef RespOwner as HeatmapView or MapGate or MapAirView;

// One /v1/heatmap request per start(), sent at once with what Place.request() gives (manual city,
// or weather position + last auto city). GPS only for the "me" mark (wantPos): it is never sent.
// Shared by HeatmapView, MapGate and MapAirView. Foreground only.
// The owner view is held weakly and the web request gets our own method(:onDone): Monkey C frees
// by reference counting and never collects cycles, so view -> fetch -> Method(view.onResp) -> view
// would leak the view (and ~8 KB per opening on 64 KB models). The owner must have onResp(code, data).
class HeatmapFetch {
    private var _view as WeakReference;
    private var _pos as Array? = null;
    private var _wantPos as Boolean;
    private var _sentPos as Boolean = false;

    function initialize(view as Object, wantPos as Boolean) {
        _view = view.weak();
        _wantPos = wantPos;
    }

    // Whether the last request carried a position (for Place.onResult).
    function sentPos() as Boolean {
        return _sentPos;
    }

    function start() as Void {
        if (_wantPos) { Position.enableLocationEvents(Position.LOCATION_ONE_SHOT, method(:onPos)); }
        var r = Place.request();
        _sentPos = r[0] != null;
        Communications.makeWebRequest(
            Api.heatmapUrl(r[0] as Array?, r[1] as String?),
            null,
            {:method => Communications.HTTP_REQUEST_METHOD_GET, :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            method(:onDone)
        );
    }

    // Web response: forwarded to the owner view if it is still alive.
    function onDone(code as Number, data as Dictionary or String or Null) as Void {
        var v = _view.get();
        if (v != null) { (v as RespOwner).onResp(code, data); }
    }

    function stop() as Void {
        if (_wantPos) { Position.enableLocationEvents(Position.LOCATION_DISABLE, method(:onPos)); }
    }

    function onPos(info as Position.Info) as Void {
        if (info.position == null) { return; }
        var deg = (info.position as Position.Location).toDegrees();
        _pos = [deg[0], deg[1]];
        WatchUi.requestUpdate();
    }

    // Last GPS fix [lat, lon], or null. Local only, never sent to the server.
    function pos() as Array? {
        return _pos;
    }
}
