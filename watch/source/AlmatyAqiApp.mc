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
        Place.applyGlance(data, Time.now().value());
        WatchUi.requestUpdate();
    }

    // The device's own map where it has one; the grid heat map elsewhere. Without glances the
    // widget starts full-screen in the widget loop, where a MapView initial view fails to start
    // the app: show the summary there, the map on SELECT.
    function getInitialView() as [Views] or [Views, InputDelegates] {
        if (startsWithSummary()) {
            return [new SummaryView(), new SummaryDelegate()];
        }
        if (WatchUi has :MapView) {
            return [new MapGate(true), new MapGateDelegate()];
        }
        return [new HeatmapView(true), new HeatmapDelegate()];
    }

    // Watches with glances (monkey.jungle excludes this on the others): glances are a setting on
    // CIQ 3.x watches, always on from CIQ 4.
    (:hasGlances)
    private function startsWithSummary() as Boolean {
        var ds = System.getDeviceSettings();
        return ds has :isGlanceModeEnabled && !ds.isGlanceModeEnabled;
    }

    // Watches without glances at all (fenix 5 family, Chronos, FR935, D2 Delta; set in
    // monkey.jungle): the runtime check above does not catch them, so a MapView would be the
    // first screen and the app would fail to start (seen in the fenix5x simulator, 2026-11-23).
    (:noGlances)
    private function startsWithSummary() as Boolean {
        return true;
    }

    (:glance)
    function getGlanceView() as [GlanceView] or [GlanceView, GlanceViewDelegate] or Null {
        return [new AirGlanceView()];
    }
}
