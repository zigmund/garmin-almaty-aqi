import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Math;
import Toybox.System;
import Toybox.WatchUi;

// First screen when widget glances are off (a setting on CIQ 3.x watches): the widget then starts
// full-screen in the widget loop, where MapView cannot be the initial view (the app fails to start).
// Shows the glance layout, larger, in a rect inscribed in the round screen; SELECT opens the map.
// Foreground only; the background service keeps the data fresh.
class SummaryView extends WatchUi.View {
    // Content rect height as a share of the screen; the width is what the circle leaves at that height.
    private const RECT_H_PCT = 42;
    private const INSET = 6;

    function initialize() {
        View.initialize();
    }

    function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();
        var sw = dc.getWidth();
        var sh = dc.getHeight();
        var d = sw < sh ? sw : sh;
        var h = d * RECT_H_PCT / 100;
        var w = Math.sqrt(d * d - h * h).toNumber() - 2 * INSET;
        if (System.getDeviceSettings().screenShape == System.SCREEN_SHAPE_RECTANGLE) { w = sw - 2 * INSET; }
        GlanceDraw.draw(dc, (sw - w) / 2, (sh - h) / 2, w, h, Graphics.FONT_TINY,
            [Graphics.FONT_NUMBER_HOT, Graphics.FONT_NUMBER_MEDIUM, Graphics.FONT_NUMBER_MILD, Graphics.FONT_MEDIUM],
            [Graphics.FONT_MEDIUM, Graphics.FONT_SMALL, Graphics.FONT_TINY]);
    }
}

// SELECT: the city map (the device's map where it has one, the grid heat map elsewhere); BACK from
// it returns here. BACK here leaves the widget.
class SummaryDelegate extends WatchUi.BehaviorDelegate {
    function initialize() {
        BehaviorDelegate.initialize();
    }

    function onSelect() as Boolean {
        if (WatchUi has :MapView) {
            var v = new MapAirView();
            WatchUi.pushView(v, new MapAirDelegate(v), WatchUi.SLIDE_LEFT);
        } else {
            WatchUi.pushView(new HeatmapView(), new HeatmapDelegate(), WatchUi.SLIDE_LEFT);
        }
        return true;
    }

    function onBack() as Boolean {
        WatchUi.popView(WatchUi.SLIDE_RIGHT);
        return true;
    }
}
