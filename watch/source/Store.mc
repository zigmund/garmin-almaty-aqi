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

    // Last place the server reported: "almaty" | "karaganda" | "out", else null.
    // Storage key stays "city" so values written by older builds still read.
    function place() as String? {
        var v = Application.Storage.getValue("city");
        if (v instanceof String && (v.equals("almaty") || v.equals("karaganda") || v.equals("out"))) { return v; }
        return null;
    }

    function setPlace(p as String) as Void {
        Application.Storage.setValue("city", p);
    }
}
