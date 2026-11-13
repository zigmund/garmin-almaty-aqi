import Toybox.Lang;
import Toybox.Test;

(:test)
function barsScaleToAtLeast150(logger as Logger) as Boolean {
    var r = Bars.heights([0, 50, 75, 150, null, 300], 30);
    var want = [0, 5, 8, 15, -1, 30];
    Test.assertEqual(r.size(), want.size());
    for (var i = 0; i < want.size(); i++) { Test.assertEqual(r[i], want[i]); }
    return true;
}

(:test)
function barsHaveOnePixelMinimum(logger as Logger) as Boolean {
    var r = Bars.heights([10, 10, 10, 10, 10, 10], 30);
    Test.assertEqual(r.size(), 6);
    for (var i = 0; i < r.size(); i++) { Test.assertEqual(r[i], 2); }
    var tiny = Bars.heights([1, 0], 30);
    Test.assertEqual(tiny[0], 1);
    Test.assertEqual(tiny[1], 0);
    return true;
}

(:test)
function barsHandleAllNullAndEmpty(logger as Logger) as Boolean {
    var r = Bars.heights([null, null, null], 30);
    Test.assertEqual(r.size(), 3);
    for (var i = 0; i < r.size(); i++) { Test.assertEqual(r[i], -1); }
    Test.assertEqual(Bars.heights([], 30).size(), 0);
    return true;
}
