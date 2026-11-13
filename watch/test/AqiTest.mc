import Toybox.Lang;
import Toybox.Test;

(:test)
function aqiCategoriesFollowEpaBands(logger as Logger) as Boolean {
    var aqis = [0, 50, 51, 100, 101, 150, 151, 200, 201, 300, 301, 500];
    var cats = [0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5];
    for (var i = 0; i < aqis.size(); i++) { Test.assertEqual(Aqi.category(aqis[i]), cats[i]); }
    Test.assertEqual(Aqi.color(0), 0x00AA00);
    Test.assertEqual(Aqi.color(5), 0x550000);
    return true;
}
