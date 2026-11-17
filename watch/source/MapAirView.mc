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
    private var _hasMarkers as Boolean = false;  // last good markers stay on screen after an error

    function initialize() {
        MapView.initialize();
        setMapMode(WatchUi.MAP_MODE_PREVIEW);
        _fetch = new HeatmapFetch(method(:onResp));
        // Frame the last known city until the first response arrives; with no
        // stored place (or "out") leave the map where it is.
        var place = Store.place();
        if (place != null && (place.equals("almaty") || place.equals("karaganda"))) {
            setArea(MapModel.cityBbox(place));
        }
    }

    function onShow() as Void {
        _state = LOADING;
        _fetch.start();
    }

    function onHide() as Void {
        _fetch.stop();
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        var r = (code == 200 && data instanceof Dictionary) ? MapModel.parse(data) : null;
        if (r == null) {
            _state = ERROR;
        } else if ((r["m"] as String).equals("nopos")) {
            clear();
            _hasMarkers = false;
            _state = NOPOS;
        } else if ((r["m"] as String).equals("out")) {
            Store.setPlace("out");
            clear();
            _hasMarkers = false;
            _state = OUT;
        } else {
            var c = (data as Dictionary)["c"];
            if (c instanceof String) { Store.setPlace(c); }
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
        var ds = System.getDeviceSettings();
        setScreenVisibleArea(0, 0, ds.screenWidth, ds.screenHeight);
    }

    function onUpdate(dc as Dc) as Void {
        if (_state == OUT || _state == NOPOS) {
            // Out of coverage / no position is explicit: no map, just the message (as on the grid heat map).
            dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
            dc.clear();
            var id = _state == OUT ? Rez.Strings.OutOfZone : Rez.Strings.NoPosShort;
            drawMessage(dc, WatchUi.loadResource(id) as String, Aqi.GRAY);
            return;
        }
        MapView.onUpdate(dc);
        if (_state == LOADING) {
            drawMessage(dc, WatchUi.loadResource(Rez.Strings.Loading) as String, Graphics.COLOR_WHITE);
        } else if (_state == ERROR && !_hasMarkers) {
            drawMessage(dc, WatchUi.loadResource(Rez.Strings.NoData) as String, Aqi.GRAY);
        }
    }

    // A centered message on a black plate, readable over the map.
    private function drawMessage(dc as Dc, s as String, color as Number) as Void {
        var font = Graphics.FONT_SMALL;
        var w = dc.getTextWidthInPixels(s, font) + 16;
        var h = dc.getFontHeight(font) + 8;
        var cx = dc.getWidth() / 2;
        var cy = dc.getHeight() / 2;
        dc.setColor(Graphics.COLOR_BLACK, Graphics.COLOR_BLACK);
        dc.fillRoundedRectangle(cx - w / 2, cy - h / 2, w, h, 6);
        dc.setColor(color, Graphics.COLOR_TRANSPARENT);
        dc.drawText(cx, cy, font, s, Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER);
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

    function onBack() as Boolean {
        if (_view.getMapMode() == WatchUi.MAP_MODE_BROWSE) {
            _view.setMapMode(WatchUi.MAP_MODE_PREVIEW);
        } else {
            WatchUi.popView(WatchUi.SLIDE_RIGHT);
        }
        return true;
    }
}
