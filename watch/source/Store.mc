import Toybox.Application;
import Toybox.Lang;

// Thin wrappers over Application.Storage.
(:glance, :background)
module Store {
    function glance() as Dictionary? {
        var v = Application.Storage.getValue("glance");
        return v instanceof Dictionary ? v : null;
    }

    function setGlance(d as Dictionary) as Void {
        Application.Storage.setValue("glance", d);
    }

    // City chosen on the watch ("almaty" | "karaganda"), else null; null removes the key.
    function manual() as String? {
        return Place.cityOrNull(Application.Storage.getValue("manual"));
    }

    function setManual(c as String?) as Void {
        if (c == null) {
            Application.Storage.deleteValue("manual");
        } else {
            Application.Storage.setValue("manual", c);
        }
    }

    // Last city the server resolved from the position. Storage key stays "city" so values
    // written by older builds still read.
    function lastAuto() as String? {
        return Place.cityOrNull(Application.Storage.getValue("city"));
    }

    function setLastAuto(c as String) as Void {
        Application.Storage.setValue("city", c);
    }

    // Whether the user was already asked to pick a city after a failed auto-detect.
    function asked() as Boolean {
        return Application.Storage.getValue("asked") == true;
    }

    function setAsked(b as Boolean) as Void {
        Application.Storage.setValue("asked", b);
    }
}
