import Toybox.Application;
import Toybox.Background;
import Toybox.System;
import Toybox.Time;
import Toybox.Lang;
import Toybox.WatchUi;

(:glance, :background)
class AirMonApp extends Application.AppBase {
    function initialize() {
        AppBase.initialize();
    }

    function onStart(state as Dictionary?) as Void {
        if (Background.getTemporalEventRegisteredTime() == null) {
            Background.registerForTemporalEvent(new Time.Duration(300));
        }
    }

    function getServiceDelegate() as [System.ServiceDelegate] {
        return [new BgService()];
    }

    function onBackgroundData(data as Application.PersistableType) as Void {
        var d = GlanceData.onReceive(data, Time.now().value());
        if (d != null) {
            Store.setGlance(d);
            var c = GlanceData.cityToStore(d);
            if (c != null) { Store.setLastCity(c); }
        }
        WatchUi.requestUpdate();
    }

    function getInitialView() as [Views] or [Views, InputDelegates] {
        return [new WatchUi.View()];
    }

    (:glance)
    function getGlanceView() as [GlanceView] or [GlanceView, GlanceViewDelegate] or Null {
        return [new AirGlanceView()];
    }
}
