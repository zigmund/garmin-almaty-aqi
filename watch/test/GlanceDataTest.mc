import Toybox.Lang;
import Toybox.Test;

(:test)
function glanceStateClassifiesPayloads(logger as Logger) as Boolean {
    Test.assertEqual(GlanceData.state(null), GlanceData.NO_DATA);
    Test.assertEqual(GlanceData.state({"m" => "out"}), GlanceData.OUT);
    Test.assertEqual(GlanceData.state({"m" => "near", "aqi" => 80, "h" => [1, 2, null]}), GlanceData.NEAR);
    Test.assertEqual(GlanceData.state({"m" => "city", "aqi" => 80, "h" => [1, 2, null]}), GlanceData.CITY);
    Test.assertEqual(GlanceData.state({"m" => "near"}), GlanceData.NO_DATA);
    Test.assertEqual(GlanceData.state({"m" => "near", "aqi" => 80}), GlanceData.NO_DATA);
    Test.assertEqual(GlanceData.state({"m" => "bogus"}), GlanceData.NO_DATA);
    Test.assertEqual(GlanceData.state({"aqi" => 80, "h" => []}), GlanceData.NO_DATA);
    return true;
}

(:test)
function glanceStalenessUsesRxTime(logger as Logger) as Boolean {
    var now = 100000;
    Test.assert(GlanceData.isStale({"rx" => now - 1801}, now));
    Test.assert(!GlanceData.isStale({"rx" => now - 1800}, now));
    Test.assert(!GlanceData.isStale({"rx" => now - 1799}, now));
    Test.assertEqual(GlanceData.ageMin({"rx" => now - 1800}, now), 30);
    Test.assertEqual(GlanceData.ageMin({"rx" => now + 500}, now), 0);
    Test.assert(GlanceData.isStale({}, now));
    Test.assert(GlanceData.ageMin({}, now) >= 9999);
    return true;
}

(:test)
function onReceiveAddsRxToMeasurements(logger as Logger) as Boolean {
    var d = GlanceData.onReceive({"m" => "near", "aqi" => 80, "h" => [1], "c" => "karaganda"}, 1234);
    Test.assert(d != null);
    Test.assertEqual(d["rx"], 1234);
    Test.assertEqual(d["aqi"], 80);
    var o = GlanceData.onReceive({"m" => "out"}, 99);
    Test.assert(o != null);
    Test.assertEqual(o["rx"], 99);
    return true;
}

(:test)
function onReceiveIgnoresErrorsAndGarbage(logger as Logger) as Boolean {
    Test.assert(GlanceData.onReceive({"err" => 500}, 1234) == null);
    Test.assert(GlanceData.onReceive(null, 1234) == null);
    Test.assert(GlanceData.onReceive({"x" => 1}, 1234) == null);
    return true;
}

(:test)
function onReceiveCityOnlyForNearAndCity(logger as Logger) as Boolean {
    var n = {"m" => "near", "aqi" => 80, "h" => [1], "c" => "karaganda"};
    var o = {"m" => "out", "c" => "almaty"};
    Test.assert(GlanceData.cityToStore(n) != null);
    Test.assertEqual(GlanceData.cityToStore(n), "karaganda");
    Test.assert(GlanceData.cityToStore({"m" => "city", "c" => "almaty"}) != null);
    Test.assert(GlanceData.cityToStore(o) == null);
    Test.assert(GlanceData.cityToStore({"m" => "near"}) == null);
    return true;
}
