import Toybox.Lang;
import Toybox.Test;

// 176 px wide, 3 px gaps: segments are (176 - 5*3) / 6 = 26 px, starting every 29 px.
(:test)
function aqiScaleSegmentGeometry(logger as Logger) as Boolean {
    var s0 = AqiScale.segment(0, 176, 3);
    var s5 = AqiScale.segment(5, 176, 3);
    Test.assertEqual(s0[0], 0);
    Test.assertEqual(s0[1], 26);
    Test.assertEqual(AqiScale.segment(1, 176, 3)[0], 29);
    Test.assertEqual(s5[0], 145);
    Test.assertEqual(s5[1], 26);
    Test.assert(s5[0] + s5[1] <= 176);
    return true;
}

(:test)
function aqiScaleMarkerAtCategoryEdges(logger as Logger) as Boolean {
    Test.assertEqual(AqiScale.markerX(0, 176, 3), 0);
    Test.assertEqual(AqiScale.markerX(50, 176, 3), 25);
    Test.assertEqual(AqiScale.markerX(51, 176, 3), 29);
    Test.assertEqual(AqiScale.markerX(100, 176, 3), 54);
    Test.assertEqual(AqiScale.markerX(301, 176, 3), 145);
    Test.assertEqual(AqiScale.markerX(500, 176, 3), 170);
    return true;
}

(:test)
function aqiScaleMarkerInterpolatesAndClamps(logger as Logger) as Boolean {
    Test.assertEqual(AqiScale.markerX(75, 176, 3), 41);   // 29 + round(24/49 * 25)
    Test.assertEqual(AqiScale.markerX(250, 176, 3), 128); // 116 + round(49/99 * 25)
    Test.assertEqual(AqiScale.markerX(600, 176, 3), 170);
    Test.assertEqual(AqiScale.markerX(-5, 176, 3), 0);
    return true;
}
