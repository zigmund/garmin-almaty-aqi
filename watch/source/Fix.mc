import Toybox.Lang;

// Position fix: [lat, lon, timeSec].
(:glance, :background)
module Fix {
    const MAX_AGE_SEC = 7200;

    function valid(f as Array?, nowSec as Number) as Boolean {
        if (!(f instanceof Array) || f.size() != 3) { return false; }
        if (!(f[0] instanceof Lang.Number || f[0] instanceof Lang.Float || f[0] instanceof Lang.Double)) { return false; }
        if (!(f[1] instanceof Lang.Number || f[1] instanceof Lang.Float || f[1] instanceof Lang.Double)) { return false; }
        var t = f[2];
        return t instanceof Lang.Number && nowSec - t <= MAX_AGE_SEC;
    }

    // Freshest valid fix of the two (info wins ties), or null.
    function pick(info as Array?, stored as Array?, nowSec as Number) as Array? {
        var a = valid(info, nowSec);
        var b = valid(stored, nowSec);
        if (a && b) { return (stored[2] as Number) > (info[2] as Number) ? stored : info; }
        if (a) { return info; }
        return b ? stored : null;
    }
}
