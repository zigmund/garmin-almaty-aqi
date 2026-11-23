import Toybox.Graphics;
import Toybox.Lang;
import Toybox.WatchUi;

// A centered message on a black plate, readable over a map. Shared by MapGate and MapAirView.
module MapMsg {
    function draw(dc as Dc, s as String, color as Number) as Void {
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

// Light screen shown before MapAirView: leaving a MapView takes ~1 s, so the one-time city question
// (and the list after "Yes") must never appear over it. Fetches /v1/heatmap itself; on a city
// response switches to the map (handing over the parsed response), otherwise shows the message
// and may ask. Foreground only.
class MapGate extends WatchUi.View {
    private enum { LOADING, OUT, NOPOS, ERROR }

    private var _state as Number = LOADING;
    private var _fetch as HeatmapFetch;
    private var _shown as Boolean = false;
    private var _root as Boolean;
    private var _leaver as Leaver? = null;

    function initialize(isRoot as Boolean) {
        View.initialize();
        _root = isRoot;
        _fetch = new HeatmapFetch(self, false);
    }

    function onShow() as Void {
        _shown = true;
        // The system pops the dialog after "Yes"; the list is pushed from here, under it.
        if (Ask.takePending()) {
            AppMenu.pushCities(1);
            return;
        }
        if (Ask.takeLeave()) {
            if (_root) {
                _state = LOADING;
                WatchUi.requestUpdate();
                _leaver = new Leaver();
                _leaver.start();
            } else {
                GlanceNow.fetch(null);
                WatchUi.popView(WatchUi.SLIDE_RIGHT);
            }
            return;
        }
        _fetch.start();
    }

    function onHide() as Void {
        _shown = false;
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        if (!_shown) { return; }
        var r = (code == 200 && data instanceof Dictionary) ? MapModel.parse(data) : null;
        if (r == null) {
            _state = ERROR;
            WatchUi.requestUpdate();
            return;
        }
        var m = r["m"] as String;
        if (m.equals("map")) {
            var c = (data as Dictionary)["c"];
            Place.onResult("city", c instanceof String ? c : null, _fetch.sentPos());
            var v = new MapAirView(r);
            WatchUi.switchToView(v, new MapAirDelegate(v), WatchUi.SLIDE_IMMEDIATE);
            return;
        }
        _state = m.equals("out") ? OUT : NOPOS;
        WatchUi.requestUpdate();
        Ask.maybe(m);
    }

    function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();
        var id = _state == OUT ? Rez.Strings.OutOfZone : _state == NOPOS ? Rez.Strings.NoPosShort
            : _state == LOADING ? Rez.Strings.Loading : Rez.Strings.NoData;
        MapMsg.draw(dc, WatchUi.loadResource(id) as String, _state == LOADING ? Graphics.COLOR_WHITE : Aqi.GRAY);
    }
}

// BACK leaves the gate (and the widget); MENU opens the app menu.
class MapGateDelegate extends WatchUi.BehaviorDelegate {
    function initialize() {
        BehaviorDelegate.initialize();
    }

    function onMenu() as Boolean {
        AppMenu.push();
        return true;
    }

    function onBack() as Boolean {
        WatchUi.popView(WatchUi.SLIDE_RIGHT);
        return true;
    }
}
