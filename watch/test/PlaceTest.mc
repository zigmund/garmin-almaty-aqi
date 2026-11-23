import Toybox.Lang;
import Toybox.Test;

(:test)
function placeCityOrNull(logger as Logger) as Boolean {
    Test.assertEqual(Place.cityOrNull("almaty"), "almaty");
    Test.assertEqual(Place.cityOrNull("karaganda"), "karaganda");
    Test.assert(Place.cityOrNull("out") == null);
    Test.assert(Place.cityOrNull("astana") == null);
    Test.assert(Place.cityOrNull(null) == null);
    Test.assert(Place.cityOrNull(5) == null);
    return true;
}

(:test)
function placeParamsManualSendsCityOnly(logger as Logger) as Boolean {
    var p = Place.params("karaganda", [43.2, 76.9], "almaty");
    Test.assert(p[0] == null);
    Test.assertEqual(p[1], "karaganda");
    return true;
}

(:test)
function placeParamsAuto(logger as Logger) as Boolean {
    var p = Place.params(null, [43.2, 76.9], "almaty");
    var pos = p[0] as Array;
    Test.assertEqual(pos[0], 43.2);
    Test.assertEqual(pos[1], 76.9);
    Test.assertEqual(p[1], "almaty");
    p = Place.params(null, null, "almaty");
    Test.assert(p[0] == null);
    Test.assertEqual(p[1], "almaty");
    p = Place.params(null, null, null);
    Test.assert(p[0] == null);
    Test.assert(p[1] == null);
    return true;
}

(:test)
function placeAutoToStore(logger as Logger) as Boolean {
    Test.assertEqual(Place.autoToStore("city", "karaganda", null, true), "karaganda");
    Test.assert(Place.autoToStore("out", null, null, true) == null);
    Test.assert(Place.autoToStore("nopos", null, null, true) == null);
    Test.assert(Place.autoToStore("city", "astana", null, true) == null);
    Test.assert(Place.autoToStore("city", null, null, true) == null);
    Test.assert(Place.autoToStore("city", "almaty", null, false) == null);
    Test.assertEqual(Place.autoToStore("city", "almaty", null, true), "almaty");
    return true;
}

(:test)
function placeAutoIgnoresManual(logger as Logger) as Boolean {
    Test.assert(Place.autoToStore("city", "almaty", "karaganda", true) == null);
    return true;
}

(:test)
function placeNeedsAsk(logger as Logger) as Boolean {
    Test.assert(Place.needsAsk("nopos", null, false));
    Test.assert(Place.needsAsk("out", null, false));
    Test.assert(!Place.needsAsk("out", null, true));
    Test.assert(!Place.needsAsk("nopos", "almaty", false));
    Test.assert(!Place.needsAsk("city", null, false));
    return true;
}
