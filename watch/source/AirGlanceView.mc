import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.WatchUi;

// Glance, laid out like the built-in HRV status one: a title line, a scale of the 6 AQI categories
// with a marker at the current value, then the AQI number with its unit and the category name
// (or the data age when stale) on one line. Drawn with dc only (32 KB limit).
(:glance)
class AirGlanceView extends WatchUi.GlanceView {
    private const SEG_GAP = 3;    // between scale segments
    private const BAR_H = 6;      // scale thickness
    private const TICK_OUT = 3;   // marker overhang above and below the scale
    private const TICK_W = 2;
    private const UNIT_GAP = 3;   // number to "AQI"
    private const WORD_GAP = 8;   // "AQI" to the category name
    // The round screen clips the right end of the lower row even inside the glance rect: keep a margin.
    private const ROW_MARGIN_PCT = 6;
    private const ROW_MARGIN_MIN = 8;

    function initialize() {
        GlanceView.initialize();
    }

    function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
        dc.clear();
        var w = dc.getWidth();
        var h = dc.getHeight();
        var small = Graphics.FONT_XTINY;
        var lineH = dc.getFontHeight(small);
        var title = WatchUi.loadResource(Rez.Strings.Title) as String;
        var d = Store.glance();
        var st = GlanceData.state(d);
        if (st == GlanceData.NOPOS) {
            // No position yet: the longest form of the message that fits the width.
            var forms = [WatchUi.loadResource(Rez.Strings.NoPos) as String, WatchUi.loadResource(Rez.Strings.NoPosShort) as String];
            var msg = forms[forms.size() - 1] as String;
            for (var i = 0; i < forms.size(); i++) {
                if (dc.getTextWidthInPixels(forms[i] as String, small) <= w) {
                    msg = forms[i] as String;
                    break;
                }
            }
            drawMessage(dc, title, msg, h, lineH);
            return;
        }
        if ((st != GlanceData.CITY && st != GlanceData.OUT) || d == null) {
            drawMessage(dc, title, WatchUi.loadResource(Rez.Strings.NoData) as String, h, lineH);
            return;
        }
        if (st == GlanceData.OUT) {
            drawMessage(dc, title, "○ " + (WatchUi.loadResource(Rez.Strings.OutOfZone) as String), h, lineH);
            return;
        }

        var now = Time.now().value();
        var stale = GlanceData.isStale(d, now);
        var aqi = d["aqi"] as Number;
        var cat = Aqi.category(aqi);

        // Word after the number, longest form first: the category name, or the age when stale
        // (the old value's category would mislead).
        var words = [];
        if (stale) {
            var mins = GlanceData.ageMin(d, now);
            var m = mins == GlanceData.NEVER_MIN ? "?" : mins.toString();
            words.add(Lang.format(WatchUi.loadResource(Rez.Strings.MinAgo) as String, [m]));
            words.add(Lang.format(WatchUi.loadResource(Rez.Strings.MinShort) as String, [m]));
        } else {
            var ids = AqiLabel.forms(cat);
            for (var i = 0; i < ids.size(); i++) { words.add(WatchUi.loadResource(ids[i]) as String); }
        }

        // Bottom row: the largest number font that fits the height, then the word in the same font
        // or in the small one, longest form that keeps the row within the width minus the margin.
        var text = aqi.toString();
        var unit = WatchUi.loadResource(Rez.Strings.Unit) as String;
        var unitW = dc.getTextWidthInPixels(unit, small);
        var scaleH = BAR_H + 2 * TICK_OUT;
        var margin = w * ROW_MARGIN_PCT / 100;
        if (margin < ROW_MARGIN_MIN) { margin = ROW_MARGIN_MIN; }
        var rowW = w - margin;
        var fonts = [Graphics.FONT_MEDIUM, Graphics.FONT_SMALL, Graphics.FONT_XTINY];
        var numFont = small;
        var wordFont = small;
        var word = words[words.size() - 1] as String;
        var found = false;
        for (var f = 0; f < fonts.size() && !found; f++) {
            var nf = fonts[f];
            if (lineH + 3 + scaleH + 2 + dc.getFontHeight(nf) > h) { continue; }
            var head = dc.getTextWidthInPixels(text, nf) + UNIT_GAP + unitW + WORD_GAP;
            var wfs = [nf, small];
            for (var k = 0; k < wfs.size() && !found; k++) {
                for (var i = 0; i < words.size(); i++) {
                    if (head + dc.getTextWidthInPixels(words[i] as String, wfs[k]) <= rowW) {
                        numFont = nf;
                        wordFont = wfs[k];
                        word = words[i] as String;
                        found = true;
                        break;
                    }
                }
            }
        }

        var numH = dc.getFontHeight(numFont);
        var top = (h - (lineH + 3 + scaleH + 2 + numH)) / 2;
        if (top < 0) { top = 0; }
        var barY = top + lineH + 3 + TICK_OUT;
        var rowY = top + lineH + 3 + scaleH + 2;
        var baseline = rowY + Graphics.getFontAscent(numFont);

        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, top, small, title, Graphics.TEXT_JUSTIFY_LEFT);

        drawScale(dc, aqi, stale, w, barY);

        var fg = stale ? Aqi.GRAY : Graphics.COLOR_WHITE;
        dc.setColor(fg, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, rowY, numFont, text, Graphics.TEXT_JUSTIFY_LEFT);
        // Unit and word sit on the number's baseline.
        var x = dc.getTextWidthInPixels(text, numFont) + UNIT_GAP;
        dc.drawText(x, baseline - Graphics.getFontAscent(small), small, unit, Graphics.TEXT_JUSTIFY_LEFT);
        x += unitW + WORD_GAP;
        dc.drawText(x, baseline - Graphics.getFontAscent(wordFont), wordFont, word, Graphics.TEXT_JUSTIFY_LEFT);
    }

    // The 6 category segments (gray when stale) and the marker at the value, with a dark gap around it.
    private function drawScale(dc as Dc, aqi as Number, stale as Boolean, w as Number, barY as Number) as Void {
        for (var i = 0; i < AqiScale.SEGMENTS; i++) {
            var s = AqiScale.segment(i, w, SEG_GAP);
            dc.setColor(stale ? Aqi.GRAY : Aqi.color(i), Graphics.COLOR_TRANSPARENT);
            dc.fillRectangle(s[0], barY, s[1], BAR_H);
        }
        var mx = AqiScale.markerX(aqi, w, SEG_GAP);
        var left = mx - TICK_W / 2;
        if (left < 0) { left = 0; }
        if (left > w - TICK_W) { left = w - TICK_W; }
        dc.setColor(Graphics.COLOR_BLACK, Graphics.COLOR_TRANSPARENT);
        dc.fillRectangle(left - 2, barY, TICK_W + 4, BAR_H);
        dc.setColor(stale ? Aqi.GRAY : Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
        dc.fillRectangle(left, barY - TICK_OUT, TICK_W, BAR_H + 2 * TICK_OUT);
    }

    // Title line plus one message line, vertically centered.
    private function drawMessage(dc as Dc, title as String, msg as String, h as Number, lineH as Number) as Void {
        var top = (h - 2 * lineH) / 2;
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, top, Graphics.FONT_XTINY, title, Graphics.TEXT_JUSTIFY_LEFT);
        dc.setColor(Aqi.GRAY, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, top + lineH, Graphics.FONT_XTINY, msg, Graphics.TEXT_JUSTIFY_LEFT);
    }
}
