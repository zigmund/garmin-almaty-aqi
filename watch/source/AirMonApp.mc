import Toybox.Application;
import Toybox.Lang;
import Toybox.WatchUi;

(:glance)
class AirMonGlanceView extends WatchUi.GlanceView {
    function initialize() {
        GlanceView.initialize();
    }
}

(:glance, :background)
class AirMonApp extends Application.AppBase {
    function initialize() {
        AppBase.initialize();
    }

    function getInitialView() as [Views] or [Views, InputDelegates] {
        return [new WatchUi.View()];
    }

    (:glance)
    function getGlanceView() as [GlanceView] or [GlanceView, GlanceViewDelegate] or Null {
        return [new AirMonGlanceView()];
    }
}
