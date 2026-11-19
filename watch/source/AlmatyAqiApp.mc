import Toybox.Application;
import Toybox.Background;
import Toybox.System;
import Toybox.Time;
import Toybox.Lang;
import Toybox.WatchUi;

(:glance, :background)
class AlmatyAqiApp extends Application.AppBase {
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

    // The device's own map where it has one; the grid heat map elsewhere. With widget glances off
    // (a setting on CIQ 3.x watches) the widget starts full-screen in the widget loop, where a
    // MapView initial view fails to start the app: show the summary there, the map on SELECT.
    function getInitialView() as [Views] or [Views, InputDelegates] {
        var ds = System.getDeviceSettings();
        if (ds has :isGlanceModeEnabled && !ds.isGlanceModeEnabled) {
            return [new SummaryView(), new SummaryDelegate()];
        }
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
