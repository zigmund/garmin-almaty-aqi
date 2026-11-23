import Toybox.Communications;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Timer;
import Toybox.WatchUi;

// Foreground /v1/glance request started by a city choice: applies the result and redraws, so the
// glance does not wait for the next background run. Maps refetch on every onShow anyway.
class GlanceNow {
    private var _hp as Boolean = false;
    var _done as Method() as Void or Null;

    // done (if not null) is called once on any response, success or error.
    static function fetch(done as Method() as Void or Null) as Void {
        var g = new GlanceNow();
        g._done = done;
        g.start();
    }

    function start() as Void {
        var p = Place.request();
        _hp = p[0] != null;
        Communications.makeWebRequest(
            Api.glanceUrl(p[0], p[1]),
            null,
            {:method => Communications.HTTP_REQUEST_METHOD_GET, :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            method(:onResp)
        );
    }

    function onResp(code as Number, data as Dictionary or String or Null) as Void {
        if (code == 200 && data instanceof Dictionary) {
            data["hp"] = _hp;
            Place.applyGlance(data, Time.now().value());
            WatchUi.requestUpdate();
        }
        var d = _done;
        _done = null;
        if (d != null) { d.invoke(); }
    }
}

// Leaves the widget after a city choice made from the question on a root screen (glance devices):
// waits for the /v1/glance response because the foreground request dies with the widget process,
// and closes anyway after 5 s (the background will refresh the glance).
class Leaver {
    private var _timer as Timer.Timer? = null;
    private var _left as Boolean = false;

    function start() as Void {
        GlanceNow.fetch(method(:leave));
        _timer = new Timer.Timer();
        (_timer as Timer.Timer).start(method(:leave), 5000, false);
    }

    function leave() as Void {
        if (_left) { return; }
        _left = true;
        if (_timer != null) {
            (_timer as Timer.Timer).stop();
            _timer = null;  // Timer holds method(:leave) -> this: break the cycle (no cycle collection)
        }
        WatchUi.popView(WatchUi.SLIDE_IMMEDIATE);
    }
}

// MENU button: the city setting and the version. Foreground only.
module AppMenu {
    function versionLabel(v as String, commit as String) as String {
        return v + " (" + commit + ")";
    }

    function cityName(c as String?) as ResourceId {
        if (c == null) { return Rez.Strings.Automatic; }
        return c.equals("karaganda") ? Rez.Strings.Karaganda : Rez.Strings.Almaty;
    }

    function push() as Void {
        var menu = new WatchUi.Menu2({:title => Rez.Strings.AppName});
        menu.addItem(new WatchUi.MenuItem(Rez.Strings.City, WatchUi.loadResource(cityName(Store.manual())) as String, :city, {}));
        menu.addItem(new WatchUi.MenuItem(Rez.Strings.Version, versionLabel(BuildInfo.VERSION, BuildInfo.COMMIT), :ver, {}));
        WatchUi.pushView(menu, new MenuDelegate(), WatchUi.SLIDE_LEFT);
    }

    // depth: how many views to pop after a choice (2 from the menu, 1 from the question).
    function pushCities(depth as Number) as Void {
        var cur = Store.manual();
        var menu = new WatchUi.Menu2({:title => Rez.Strings.City});
        var ids = [:auto, :almaty, :karaganda];
        var codes = [null, "almaty", "karaganda"];
        var focus = 0;
        for (var i = 0; i < ids.size(); i++) {
            var code = codes[i] as String?;
            var on = (code == null) ? cur == null : code.equals(cur);
            if (on) { focus = i; }
            menu.addItem(new WatchUi.MenuItem(cityName(code), on ? WatchUi.loadResource(Rez.Strings.Selected) as String : null, ids[i], {}));
        }
        menu.setFocus(focus);
        // From the question: no slide, so the screen under the closed dialog is not shown while the
        // list animates in.
        WatchUi.pushView(menu, new CitiesDelegate(depth), depth == 1 ? WatchUi.SLIDE_IMMEDIATE : WatchUi.SLIDE_LEFT);
    }
}

class MenuDelegate extends WatchUi.Menu2InputDelegate {
    function initialize() {
        Menu2InputDelegate.initialize();
    }

    function onSelect(item as WatchUi.MenuItem) as Void {
        if (item.getId() == :city) { AppMenu.pushCities(2); }
    }
}

class CitiesDelegate extends WatchUi.Menu2InputDelegate {
    private var _depth as Number;

    function initialize(depth as Number) {
        Menu2InputDelegate.initialize();
        _depth = depth;
    }

    function onSelect(item as WatchUi.MenuItem) as Void {
        var id = item.getId();
        Store.setManual(id == :almaty ? "almaty" : (id == :karaganda ? "karaganda" : null));
        if (_depth == 1) {
            // From the question: the view that hosts it decides where to go (see Ask.takeLeave);
            // it starts the glance request itself.
            Ask.setLeave();
            WatchUi.popView(WatchUi.SLIDE_IMMEDIATE);
            return;
        }
        GlanceNow.fetch(null);
        for (var i = 0; i < _depth; i++) {
            WatchUi.popView(WatchUi.SLIDE_IMMEDIATE);
        }
    }
}

// One-time question after a failed auto-detect (nopos / out): offer the city list.
module Ask {
    function maybe(m as String?) as Void {
        if (!Place.needsAsk(m, Store.manual(), Store.asked())) { return; }
        Store.setAsked(true);
        WatchUi.pushView(new WatchUi.Confirmation(WatchUi.loadResource(Rez.Strings.AskCity) as String), new AskDelegate(), WatchUi.SLIDE_IMMEDIATE);
    }

    // "Yes" only sets this flag: the system pops the top view after onResponse, so the list is
    // pushed from the onShow of the view under the dialog (takePending there).
    var _pending as Boolean = false;

    function setPending() as Void {
        _pending = true;
    }

    function takePending() as Boolean {
        var p = _pending;
        _pending = false;
        return p;
    }

    // Set when a city was chosen from the question: the view under the list returns to where the
    // user came from (summary / glance) instead of showing the map.
    var _leave as Boolean = false;

    function setLeave() as Void {
        _leave = true;
    }

    function takeLeave() as Boolean {
        var p = _leave;
        _leave = false;
        return p;
    }
}

class AskDelegate extends WatchUi.ConfirmationDelegate {
    function initialize() {
        ConfirmationDelegate.initialize();
    }

    function onResponse(response as WatchUi.Confirm) as Boolean {
        // A list pushed here would be popped instead of the dialog (fenix5x simulator, 2026-11-23).
        if (response == WatchUi.CONFIRM_YES) { Ask.setPending(); }
        return true;
    }
}
