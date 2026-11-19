import Toybox.Graphics;
import Toybox.Lang;
import Toybox.WatchUi;

// Glance: the shared glance layout (GlanceDraw) over the whole glance rect.
(:glance)
class AirGlanceView extends WatchUi.GlanceView {
    function initialize() {
        GlanceView.initialize();
    }

    function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_TRANSPARENT);
        dc.clear();
        GlanceDraw.draw(dc, 0, 0, dc.getWidth(), dc.getHeight(), Graphics.FONT_XTINY,
            [Graphics.FONT_MEDIUM, Graphics.FONT_SMALL, Graphics.FONT_XTINY], null);
    }
}
