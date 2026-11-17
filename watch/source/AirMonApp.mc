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
            var p = GlanceData.placeToStore(d);
            if (p != null) { Store.setPlace(p); }
        }
        WatchUi.requestUpdate();
    }

    // The device's own map where it has one; the grid heat map elsewhere.
    function getInitialView() as [Views] or [Views, InputDelegates] {
        if (WatchUi has :MapView) {
            var v = new MapAirView();
            return [v, new MapAirDelegate(v)];
        }
        return [new HeatmapView(), new HeatmapDelegate()];
    }

    (:glance)
    function getGlanceView() as [GlanceView] or [GlanceView, GlanceViewDelegate] or Null {
        return [new AirGlanceView()];
    }
}
