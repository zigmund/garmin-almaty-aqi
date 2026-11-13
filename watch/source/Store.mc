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

    function lastCity() as String {
        var v = Application.Storage.getValue("city");
        return v instanceof String ? v : "almaty";
    }

    function setLastCity(c as String) as Void {
        Application.Storage.setValue("city", c);
    }

    function fix() as Array? {
        var v = Application.Storage.getValue("fix");
        return v instanceof Array ? v : null;
    }

    function setFix(f as Array) as Void {
        Application.Storage.setValue("fix", f);
    }
}
