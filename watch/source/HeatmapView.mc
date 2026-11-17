import Toybox.Graphics;
import Toybox.Lang;
import Toybox.System;
import Toybox.Time;
import Toybox.WatchUi;

// Render-time log: debug builds only, nothing in release.
module HeatmapLog {
    (:debug)
    function renderMs(ms as Number) as Void {
        System.println("heatmap onUpdate " + ms + " ms");
    }

    (:release)
    function renderMs(ms as Number) as Void {
    }
}

// Full-screen city heatmap. Runs in the full widget process only (not glance/background).
class HeatmapView extends WatchUi.View {
    private const CROSS_HALF = 3;

    private enum { LOADING, OUT, ERROR, MAP }

    private var _state as Number = LOADING;
    private var _fetch as HeatmapFetch;

    // Last good map, kept in memory only (never persisted).
    private var _grid as ByteArray? = null;
    private var _gw as Number = 0;
    private var _gh as Number = 0;
    private var _stations as Array = [];  // [[fx, fy, cat], ...]
    private var _me as Array? = null;     // [fx, fy]
    private var _time as String? = null;  // HH:MM of receipt

    function initialize() {
        View.initialize();
        _fetch = new HeatmapFetch(method(:onResp));
    }

    function onShow() as Void {
        _state = LOADING;
        _fetch.start();
    }

    function onHide() as Void {
        _fetch.stop();
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        if (code != 200 || !(data instanceof Dictionary)) {
            _state = ERROR;
        } else {
            _state = accept(data);
        }
        WatchUi.requestUpdate();
    }

    // Applies a 200 payload; returns the resulting state (ERROR leaves the last good map alone).
    private function accept(data as Dictionary) as Number {
        var m = data["m"];
        if (!(m instanceof String)) { return ERROR; }
        if (m.equals("out")) {
            // Out of zone is explicit: drop the held map so a later error shows NoData.
            _grid = null;
            _stations = [];
            _me = null;
            _time = null;
            return OUT;
        }
        if (!m.equals("near") && !m.equals("city")) { return ERROR; }
        var c = data["c"];
        if (c instanceof String) { Store.setLastCity(c); }

        var w = data["w"];
        var h = data["h"];
        var g = data["g"];
        var bbox = data["bbox"];
        if (!(w instanceof Number) || !(h instanceof Number) || !(g instanceof String)) { return ERROR; }
        if (!(bbox instanceof Array) || bbox.size() != 4) { return ERROR; }
        var grid = HeatmapModel.decode(g, w, h);
        if (grid == null) { return ERROR; }

        var stations = [];
        var s = data["s"];
        if (s instanceof Array) {
            for (var i = 0; i < s.size(); i++) {
                var p = s[i];
                if (!(p instanceof Array) || p.size() < 3) { continue; }
                try {
                    var fx = (p[1].toFloat() - bbox[0].toFloat()) / (bbox[2].toFloat() - bbox[0].toFloat());
                    var fy = (bbox[3].toFloat() - p[0].toFloat()) / (bbox[3].toFloat() - bbox[1].toFloat());
                    if (inUnit(fx) && inUnit(fy) && p[2] instanceof Number) { stations.add([fx, fy, p[2]]); }
                } catch (ex) {
                    // malformed station: skip it
                }
            }
        }

        var you = null;
        var mv = data["me"];
        if (mv instanceof Array && mv.size() == 2 && (mv[0] instanceof Number || mv[0] instanceof Float || mv[0] instanceof Double)
            && (mv[1] instanceof Number || mv[1] instanceof Float || mv[1] instanceof Double)) {
            var mx = mv[0].toFloat();
            var my = mv[1].toFloat();
            if (inUnit(mx) && inUnit(my)) { you = [mx, my]; }
        }

        // Header time is the data time (receipt time minus the proxy's age), local clock.
        var dt = Time.Gregorian.info(new Time.Moment(GlanceData.dataTime(data["a"], Time.now().value())), Time.FORMAT_SHORT);
        _grid = grid;
        _gw = w;
        _gh = h;
        _stations = stations;
        _me = you;
        _time = dt.hour.format("%02d") + ":" + dt.min.format("%02d");
        return MAP;
    }

    private function inUnit(f as Float) as Boolean {
        return f >= 0.0 && f <= 1.0;
    }

    function onUpdate(dc as Dc) as Void {
        var t0 = System.getTimer();
        var w = dc.getWidth();
        var h = dc.getHeight();
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();

        if (_state == OUT) {
            drawCentered(dc, WatchUi.loadResource(Rez.Strings.OutOfZone) as String, Aqi.GRAY);
        } else if (_state == LOADING) {
            drawHeader(dc, false);
            drawCentered(dc, WatchUi.loadResource(Rez.Strings.Loading) as String, Graphics.COLOR_WHITE);
        } else if (_grid == null) {
            drawCentered(dc, WatchUi.loadResource(Rez.Strings.NoData) as String, Aqi.GRAY);
        } else {
            var hdrBottom = drawHeader(dc, true);
            drawMap(dc, w, h, hdrBottom);
        }
        HeatmapLog.renderMs(System.getTimer() - t0);
    }

    // "AQI N" in its category colour plus the data time; returns the header's bottom y.
    private function drawHeader(dc as Dc, withTime as Boolean) as Number {
        var font = Graphics.FONT_XTINY;
        var fh = dc.getFontHeight(font);
        var top = dc.getHeight() / 20 + 2;
        var gd = Store.glance();
        var st = GlanceData.state(gd);
        var aqiText = null;
        var aqiColor = Aqi.GRAY;
        if ((st == GlanceData.NEAR || st == GlanceData.CITY) && gd != null) {
            var n = gd["aqi"] as Number;
            aqiText = "AQI " + n;
            aqiColor = GlanceData.isStale(gd, Time.now().value()) ? Aqi.GRAY : Aqi.color(Aqi.category(n));
        }
        var timeText = withTime ? _time : null;
        var gap = "  ";
        var wa = aqiText != null ? dc.getTextWidthInPixels(aqiText, font) : 0;
        var wt = timeText != null ? dc.getTextWidthInPixels(timeText, font) : 0;
        var wg = (aqiText != null && timeText != null) ? dc.getTextWidthInPixels(gap, font) : 0;
        var x = (dc.getWidth() - (wa + wg + wt)) / 2;
        if (aqiText != null) {
            dc.setColor(aqiColor, Graphics.COLOR_TRANSPARENT);
            dc.drawText(x, top, font, aqiText, Graphics.TEXT_JUSTIFY_LEFT);
        }
        if (timeText != null) {
            dc.setColor(Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
            dc.drawText(x + wa + wg, top, font, timeText, Graphics.TEXT_JUSTIFY_LEFT);
        }
        return top + fh;
    }

    private function drawMap(dc as Dc, w as Number, h as Number, hdrBottom as Number) as Void {
        var grid = _grid as ByteArray;
        var m = w < h ? w : h;
        // Square inscribed in the circle, with a small margin; never taller than the room under the header.
        var side = (m * 11 / 10) * 100 / 141;
        var room = h - hdrBottom - (h - m) / 2 - 2;
        if (side > room) { side = room; }
        if (m - 2 * hdrBottom > 0 && side > m - 2 * hdrBottom) { side = m - 2 * hdrBottom; }
        var ox = (w - side) / 2;
        var oy = (h - side) / 2 + hdrBottom / 4;
        if (oy < hdrBottom) { oy = hdrBottom; }

        for (var j = 0; j < _gh; j++) {
            for (var i = 0; i < _gw; i++) {
                var cat = grid[j * _gw + i];
                if (cat > 5) { continue; }
                var r = HeatmapModel.cellRect(i, j, _gw, _gh, side, ox, oy);
                dc.setColor(Aqi.color(cat), Graphics.COLOR_TRANSPARENT);
                dc.fillRectangle(r[0], r[1], r[2], r[3]);
            }
        }

        for (var k = 0; k < _stations.size(); k++) {
            var s = _stations[k] as Array;
            var p = HeatmapModel.toPx([s[0], s[1]], side, ox, oy);
            var sc = s[2] as Number;
            dc.setColor(sc <= 5 ? Aqi.color(sc) : Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
            dc.fillCircle(p[0], p[1], 2);
            dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
            dc.drawCircle(p[0], p[1], 2);
        }

        if (_me != null) {
            var q = HeatmapModel.toPx(_me as Array, side, ox, oy);
            dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
            dc.drawLine(q[0] - CROSS_HALF, q[1], q[0] + CROSS_HALF, q[1]);
            dc.drawLine(q[0], q[1] - CROSS_HALF, q[0], q[1] + CROSS_HALF);
        }
    }

    private function drawCentered(dc as Dc, s as String, color as Number) as Void {
        dc.setColor(color, Graphics.COLOR_TRANSPARENT);
        dc.drawText(dc.getWidth() / 2, dc.getHeight() / 2, Graphics.FONT_SMALL, s, Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER);
    }
}
