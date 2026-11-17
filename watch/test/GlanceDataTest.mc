import Toybox.Lang;
import Toybox.Test;

(:test)
function glanceStateClassifiesPayloads(logger as Logger) as Boolean {
    Test.assertEqual(GlanceData.state(null), GlanceData.NO_DATA);
    Test.assertEqual(GlanceData.state({"m" => "out"}), GlanceData.OUT);
    Test.assertEqual(GlanceData.state({"m" => "nopos"}), GlanceData.NOPOS);
    Test.assertEqual(GlanceData.state({"m" => "city", "aqi" => 80}), GlanceData.CITY);
    Test.assertEqual(GlanceData.state({"m" => "near", "aqi" => 80, "h" => []}), GlanceData.NO_DATA);
    Test.assertEqual(GlanceData.state({"m" => "city"}), GlanceData.NO_DATA);
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
function stalenessUsesDataTimeFromAge(logger as Logger) as Boolean {
    var now = 100000;
    // Data 2400 s old when received just now: stale although rx is fresh.
    var d = GlanceData.onReceive({"m" => "near", "aqi" => 80, "h" => [1], "a" => 2400}, now);
    Test.assert(d != null);
    Test.assertEqual(d["dt"], now - 2400);
    Test.assert(GlanceData.isStale(d, now));
    Test.assertEqual(GlanceData.ageMin(d, now), 40);
    // Fresh data received a while ago: age grows from the data time.
    var f = GlanceData.onReceive({"m" => "near", "aqi" => 80, "h" => [1], "a" => 120}, now);
    Test.assert(!GlanceData.isStale(f, now));
    Test.assertEqual(GlanceData.ageMin(f, now), 2);
    Test.assert(GlanceData.isStale(f, now + 1700));
    return true;
}

(:test)
function missingOrBadAgeFallsBackToRx(logger as Logger) as Boolean {
    var now = 100000;
    var d = GlanceData.onReceive({"m" => "near", "aqi" => 80, "h" => [1]}, now);
    Test.assertEqual(d["dt"], now);
    Test.assert(!GlanceData.isStale(d, now));
    var s = GlanceData.onReceive({"m" => "near", "aqi" => 80, "h" => [1], "a" => "x"}, now);
    Test.assertEqual(s["dt"], now);
    Test.assertEqual(GlanceData.dataTime(null, now), now);
    Test.assertEqual(GlanceData.dataTime(60, now), now - 60);
    Test.assertEqual(GlanceData.dataTime(-5, now), now);
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
function placeToStoreRemembersOut(logger as Logger) as Boolean {
    Test.assertEqual(GlanceData.placeToStore({"m" => "out"}), "out");
    Test.assertEqual(GlanceData.placeToStore({"m" => "city", "c" => "karaganda"}), "karaganda");
    Test.assert(GlanceData.placeToStore({"m" => "nopos"}) == null);
    Test.assert(GlanceData.placeToStore({"m" => "city"}) == null);
    return true;
}

(:test)
function ageShowsMinutesThenWholeHours(logger as Logger) as Boolean {
    var cases = [[0, "0", false], [59, "59", false], [60, "1", true], [119, "1", true], [120, "2", true]];
    for (var i = 0; i < cases.size(); i++) {
        var c = cases[i] as Array;
        var a = GlanceData.ageParts(c[0] as Number);
        Test.assertEqual(a[0], c[1]);
        Test.assertEqual(a[1], c[2]);
    }
    var n = GlanceData.ageParts(GlanceData.NEVER_MIN);
    Test.assertEqual(n[0], "?");
    Test.assertEqual(n[1], false);
    return true;
}

(:test)
function splitTwoBalancesLines(logger as Logger) as Boolean {
    var cases = [
        ["Unhealthy (sensitive)", "Unhealthy", "(sensitive)"],
        ["Unhealthy for sensitive groups", "Unhealthy for", "sensitive groups"],
        ["Нездоровый для чувствительных групп", "Нездоровый для", "чувствительных групп"]
    ];
    for (var i = 0; i < cases.size(); i++) {
        var c = cases[i] as Array;
        var p = GlanceData.splitTwo(c[0] as String) as Array;
        Test.assertEqual(p[0], c[1]);
        Test.assertEqual(p[1], c[2]);
    }
    Test.assert(GlanceData.splitTwo("Good") == null);
    return true;
}
