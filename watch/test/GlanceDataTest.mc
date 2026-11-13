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
