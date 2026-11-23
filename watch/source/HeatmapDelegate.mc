import Toybox.Lang;
import Toybox.WatchUi;

// Back leaves the heatmap; no other input.
class HeatmapDelegate extends WatchUi.BehaviorDelegate {
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
