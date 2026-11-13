import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.WatchUi;

// Glance: AQI number on the left, 6 hourly bars on the right. Drawn with dc only (32 KB limit).
(:glance)
class AirGlanceView extends WatchUi.GlanceView {
    private const GAP = 3;

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

        var hist = d["h"] as Array;
        var barsX = barsLeft(hist.size(), w);
        var availW = barsX - h / 8;  // left column: up to the bars minus a margin

        // Caption: one line, "city" or "N min ago" (age wins when stale: it is the longer one).
        var sub = null;
        if (stale) {
            sub = Lang.format(WatchUi.loadResource(Rez.Strings.MinAgo) as String, [GlanceData.ageMin(d, now)]);
        } else if (st == GlanceData.CITY) {
            sub = WatchUi.loadResource(Rez.Strings.City) as String;
        }
        var subFont = Graphics.FONT_XTINY;
        var subH = sub == null ? 0 : dc.getFontHeight(subFont);
        if (sub != null && dc.getTextWidthInPixels(sub, subFont) > availW) { sub = null; subH = 0; }

        // Largest number font that fits both the height (with caption) and the column width.
        var text = aqi.toString();
        var fonts = [Graphics.FONT_NUMBER_MEDIUM, Graphics.FONT_NUMBER_MILD, Graphics.FONT_MEDIUM];
        var numFont = fonts[fonts.size() - 1];
        for (var i = 0; i < fonts.size(); i++) {
            if (dc.getFontHeight(fonts[i]) + subH <= h && dc.getTextWidthInPixels(text, fonts[i]) <= availW) {
                numFont = fonts[i];
                break;
            }
        }
        var numH = dc.getFontHeight(numFont);
        var top = (h - numH - subH) / 2;
        if (top < 0) { top = 0; }
        dc.setColor(color, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, top, numFont, text, Graphics.TEXT_JUSTIFY_LEFT);
        if (sub != null) {
            dc.setColor(Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
            dc.drawText(0, top + numH, subFont, sub, Graphics.TEXT_JUSTIFY_LEFT);
        }

        drawBars(dc, hist, stale, w, h);
    }

    // Bar geometry: the bars take the right ~45% of the width, with a fixed gap.
    private function barWidth(n as Number, w as Number) as Number {
        var bw = (w * 45 / 100 - (n - 1) * GAP) / n;
        return bw < 2 ? 2 : bw;
    }

    private function barsLeft(n as Number, w as Number) as Number {
        return n == 0 ? w : w - (barWidth(n, w) * n + GAP * (n - 1));
    }

    // Right ~45% of the width, bars grow up from the bottom margin.
    private function drawBars(dc as Dc, hist as Array, stale as Boolean, w as Number, h as Number) as Void {
        var n = hist.size();
        if (n == 0) { return; }
        var margin = h / 8;
        var maxPx = h - 2 * margin;
        var gap = GAP;
        var bw = barWidth(n, w);
        var x = barsLeft(n, w);
        var base = h - margin;
        var hs = Bars.heights(hist, maxPx);
        for (var i = 0; i < n; i++) {
            var px = hs[i];
            if (px < 0) {
                dc.setColor(Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
                dc.fillRectangle(x, base - 2, bw, 2);
            } else {
                var cat = Aqi.category(hist[i] as Number);
                // Stale data is gray (spec: older than 30 min), bars included.
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
