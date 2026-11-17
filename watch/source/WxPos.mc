import Toybox.Lang;
import Toybox.Weather;

// Position from the weather observation (works in the background process; Position.getInfo does not
// have to). No age check: the server decides what a stale position is worth.
(:background)
module WxPos {
    // [lat, lon] in degrees (Float), or null when weather or its position is unavailable.
    function read() as Array? {
        try {
            if (!(Toybox has :Weather)) { return null; }
            var cc = Weather.getCurrentConditions();
            if (cc == null) { return null; }
            var loc = cc.observationLocationPosition;
            if (loc == null) { return null; }
            var deg = loc.toDegrees();
            return [deg[0].toFloat(), deg[1].toFloat()];
        } catch (ex) {
            return null;
        }
    }
}
