import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Position;
import Toybox.System;
import Toybox.WatchUi;

// Full-screen city map on devices with Garmin maps: the device's own map with one checkerboard
// marker per station (colored by AQI category); the map shows the user's position itself.
// Same /v1/heatmap data as HeatmapView, which remains the screen on devices without MapView.
// Foreground only.
class MapAirView extends WatchUi.MapView {
    private enum { LOADING, OUT, NOPOS, ERROR, MAP }

    private var _state as Number = LOADING;
    private var _fetch as HeatmapFetch;
    private var _shown as Boolean = false;       // a response after onHide must not touch the view
    private var _hasMarkers as Boolean = false;  // last good markers stay on screen after an error
    private var _hasCity as Boolean = false;     // the map area frames a city (stored or from a response)

    private var _skipFetch as Boolean = false;   // the first onShow uses the response MapGate handed over

    // first: the parsed city response from MapGate, or null (then the first onShow fetches).
    function initialize(first as Dictionary?) {
        MapView.initialize();
        setMapMode(WatchUi.MAP_MODE_PREVIEW);
        var ds = System.getDeviceSettings();
        setScreenVisibleArea(0, 0, ds.screenWidth, ds.screenHeight);
        _fetch = new HeatmapFetch(self, false);
        // MapView throws at start without a map area, so always set one: the last known city, or
        // Almaty as a placeholder that is not drawn (_hasCity stays false) until a city response.
        var place = Place.city();
        _hasCity = place != null;
        setArea(MapModel.cityBbox(_hasCity ? place as String : "almaty"));
        if (first != null) {
            showMarkers(first);
            _state = MAP;
            _skipFetch = true;
        }
    }

    function onShow() as Void {
        _shown = true;
        // Keep the last screen while refetching (LOADING only before the first response): coming
        // back from the question or the menu must not flash "Loading".
        if (_skipFetch) {
            _skipFetch = false;
            return;
        }
        _fetch.start();
    }

    function onHide() as Void {
        _shown = false;
        _fetch.stop();
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        if (!_shown) { return; }
        var r = (code == 200 && data instanceof Dictionary) ? MapModel.parse(data) : null;
        if (r == null) {
            _state = ERROR;
        } else if ((r["m"] as String).equals("nopos")) {
            clear();
            _hasMarkers = false;
            _state = NOPOS;
        } else if ((r["m"] as String).equals("out")) {
            clear();
            _hasMarkers = false;
            _state = OUT;
        } else {
            var c = (data as Dictionary)["c"];
            Place.onResult("city", c instanceof String ? c : null, _fetch.sentPos());
            showMarkers(r);
            _state = MAP;
        }
        WatchUi.requestUpdate();
    }

    private function showMarkers(r as Dictionary) as Void {
        var icons = [
            Rez.Drawables.Marker0, Rez.Drawables.Marker1, Rez.Drawables.Marker2,
            Rez.Drawables.Marker3, Rez.Drawables.Marker4, Rez.Drawables.Marker5, Rez.Drawables.MarkerGray
        ];
        var bitmaps = new Array<BitmapResource?>[icons.size()];
        var markers = [];
        var st = r["st"] as Array;
        for (var i = 0; i < st.size(); i++) {
            var p = st[i] as Array;
            var k = p[2] as Number;
            if (bitmaps[k] == null) { bitmaps[k] = WatchUi.loadResource(icons[k]) as BitmapResource; }
            markers.add(marker(p[0] as Float, p[1] as Float, bitmaps[k] as BitmapResource));
        }
        clear();
        setMapMarker(markers);
        setArea(r["bbox"] as Array);
        _hasCity = true;
        _hasMarkers = true;
    }

    // Marker centered on the point.
    private function marker(lat as Float, lon as Float, icon as BitmapResource) as WatchUi.MapMarker {
        var m = new WatchUi.MapMarker(new Position.Location({:latitude => lat, :longitude => lon, :format => :degrees}));
        m.setIcon(icon, icon.getWidth() / 2, icon.getHeight() / 2);
        return m;
    }

    // bbox [w, s, e, n] in degrees, drawn over the whole screen.
    private function setArea(bbox as Array) as Void {
        var tl = new Position.Location({:latitude => bbox[3].toFloat(), :longitude => bbox[0].toFloat(), :format => :degrees});
        var br = new Position.Location({:latitude => bbox[1].toFloat(), :longitude => bbox[2].toFloat(), :format => :degrees});
        setMapVisibleArea(tl, br);
    }

    function onUpdate(dc as Dc) as Void {
        if (_state == OUT || _state == NOPOS || !_hasCity) {
            // Out of coverage / no position is explicit: no map, just the message (as on the grid heat map).
            // Same while no city is known: the placeholder area is not shown.
            dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
            dc.clear();
            var id = _state == OUT ? Rez.Strings.OutOfZone : _state == NOPOS ? Rez.Strings.NoPosShort
                : _state == LOADING ? Rez.Strings.Loading : Rez.Strings.NoData;
            MapMsg.draw(dc, WatchUi.loadResource(id) as String, _state == LOADING ? Graphics.COLOR_WHITE : Aqi.GRAY);
            return;
        }
        MapView.onUpdate(dc);
        if (_state == LOADING) {
            MapMsg.draw(dc, WatchUi.loadResource(Rez.Strings.Loading) as String, Graphics.COLOR_WHITE);
        } else if (_state == ERROR && !_hasMarkers) {
            MapMsg.draw(dc, WatchUi.loadResource(Rez.Strings.NoData) as String, Aqi.GRAY);
        }
    }
}

// SELECT: browse mode (the system pans/zooms with the buttons). BACK: back to the city preview,
// then out of the app.
class MapAirDelegate extends WatchUi.BehaviorDelegate {
    private var _view as MapAirView;

    function initialize(view as MapAirView) {
        BehaviorDelegate.initialize();
        _view = view;
    }

    function onSelect() as Boolean {
        _view.setMapMode(WatchUi.MAP_MODE_BROWSE);
        return true;
    }

    function onMenu() as Boolean {
        AppMenu.push();
        return true;
    }

    function onBack() as Boolean {
        if (_view.getMapMode() == WatchUi.MAP_MODE_BROWSE) {
            _view.setMapMode(WatchUi.MAP_MODE_PREVIEW);
        } else {
            WatchUi.popView(WatchUi.SLIDE_RIGHT);
        }
        return true;
    }
}
