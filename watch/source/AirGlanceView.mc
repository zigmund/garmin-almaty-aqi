import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.WatchUi;

// Glance: AQI number on the left, 6 hourly bars on the right. Drawn with dc only (32 KB limit).
(:glance)
class AirGlanceView extends WatchUi.GlanceView {
    function initialize() {
        GlanceView.initialize();
    }

    function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
        dc.clear();
        var w = dc.getWidth();
        var h = dc.getHeight();
        var d = Store.glance();
        var st = GlanceData.state(d);
        if (st == GlanceData.NO_DATA || d == null) {
            drawText(dc, WatchUi.loadResource(Rez.Strings.NoData) as String, Aqi.GRAY, h / 2);
            return;
        }
        if (st == GlanceData.OUT) {
            drawText(dc, "○ " + (WatchUi.loadResource(Rez.Strings.OutOfZone) as String), Aqi.GRAY, h / 2);
            return;
        }

        var now = Time.now().value();
        var stale = GlanceData.isStale(d, now);
        var aqi = d["aqi"] as Number;
        var color = stale ? Aqi.GRAY : Aqi.color(Aqi.category(aqi));

        // Left: number, with a small caption underneath when there is one.
        var sub = null;
        if (st == GlanceData.CITY) { sub = WatchUi.loadResource(Rez.Strings.City) as String; }
        if (stale) {
            var ago = Lang.format(WatchUi.loadResource(Rez.Strings.MinAgo) as String, [GlanceData.ageMin(d, now)]);
            sub = (sub == null) ? ago : sub + " · " + ago;
        }
        var numFont = Graphics.FONT_NUMBER_MEDIUM;
        var subFont = Graphics.FONT_XTINY;
        var numH = dc.getFontHeight(numFont);
        var subH = sub == null ? 0 : dc.getFontHeight(subFont);
        var top = (h - numH - subH) / 2;
        if (top < 0) { top = 0; }
        dc.setColor(color, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, top, numFont, aqi.toString(), Graphics.TEXT_JUSTIFY_LEFT);
        if (sub != null) {
            dc.setColor(Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
            dc.drawText(0, top + numH, subFont, sub, Graphics.TEXT_JUSTIFY_LEFT);
        }

        drawBars(dc, d["h"] as Array, stale, w, h);
    }

    // Right ~45% of the width, bars grow up from the bottom margin.
    private function drawBars(dc as Dc, hist as Array, stale as Boolean, w as Number, h as Number) as Void {
        var n = hist.size();
        if (n == 0) { return; }
        var margin = h / 8;
        var maxPx = h - 2 * margin;
        var gap = 3;
        var area = w * 45 / 100;
        var bw = (area - (n - 1) * gap) / n;
        if (bw < 2) { bw = 2; }
        var x = w - (bw * n + gap * (n - 1));
        var base = h - margin;
        var hs = Bars.heights(hist, maxPx);
        for (var i = 0; i < n; i++) {
            var px = hs[i];
            if (px < 0) {
                dc.setColor(Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
                dc.fillRectangle(x, base - 2, bw, 2);
            } else {
                var cat = Aqi.category(hist[i] as Number);
                dc.setColor(stale ? Aqi.GRAY : Aqi.color(cat), Graphics.COLOR_TRANSPARENT);
                dc.fillRectangle(x, base - px, bw, px);
            }
            x += bw + gap;
        }
    }

    private function drawText(dc as Dc, s as String, color as Number, y as Number) as Void {
        dc.setColor(color, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, y, Graphics.FONT_XTINY, s, Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
    }
}
