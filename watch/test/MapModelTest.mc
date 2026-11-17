import Toybox.Lang;
import Toybox.Test;

function near(a, b) as Boolean {
    var d = a.toFloat() - b.toFloat();
    return d < 0.0001 && d > -0.0001;
}

(:test)
function mapMarkerIndexPerCategory(logger as Logger) as Boolean {
    for (var c = 0; c <= 5; c++) { Test.assertEqual(MapModel.markerIndex(c), c); }
    Test.assertEqual(MapModel.markerIndex(6), MapModel.GRAY);
    Test.assertEqual(MapModel.markerIndex(255), MapModel.GRAY);
    Test.assertEqual(MapModel.markerIndex(-1), MapModel.GRAY);
    Test.assertEqual(MapModel.markerIndex(null), MapModel.GRAY);
    Test.assertEqual(MapModel.markerIndex("2"), MapModel.GRAY);
    return true;
}

(:test)
function mapCityBboxMatchesProxy(logger as Logger) as Boolean {
    var a = MapModel.cityBbox("almaty");
    Test.assert(near(a[0], 76.6) && near(a[1], 43.14) && near(a[2], 77.26) && near(a[3], 43.5));
    var k = MapModel.cityBbox("karaganda");
    Test.assert(near(k[0], 72.9) && near(k[1], 49.7) && near(k[2], 73.3) && near(k[3], 49.95));
    var u = MapModel.cityBbox("astana");  // unknown -> Almaty
    Test.assert(near(u[0], 76.6) && near(u[3], 43.5));
    return true;
}

(:test)
function mapFracToLatLon(logger as Logger) as Boolean {
    var bb = [76.6, 43.14, 77.26, 43.5];
    var tl = MapModel.fracToLatLon(bb, 0.0, 0.0);
    Test.assert(near(tl[0], 43.5) && near(tl[1], 76.6));
    var c = MapModel.fracToLatLon(bb, 0.5, 0.5);
    Test.assert(near(c[0], 43.32) && near(c[1], 76.93));
    var br = MapModel.fracToLatLon(bb, 1.0, 1.0);
    Test.assert(near(br[0], 43.14) && near(br[1], 77.26));
    return true;
}

(:test)
function mapParseOutAndErrors(logger as Logger) as Boolean {
    var o = MapModel.parse({"m" => "out"});
    Test.assert(o != null && (o["m"] as String).equals("out"));
    Test.assert(MapModel.parse({}) == null);
    Test.assert(MapModel.parse({"m" => 5}) == null);
    Test.assert(MapModel.parse({"m" => "near"}) == null);                       // no bbox
    Test.assert(MapModel.parse({"m" => "near", "bbox" => [1, 2, 3]}) == null);  // short bbox
    Test.assert(MapModel.parse({"m" => "weird", "bbox" => [76.6, 43.14, 77.26, 43.5]}) == null);
    return true;
}

(:test)
function mapParseStationsAndMe(logger as Logger) as Boolean {
    var d = {
        "m" => "city", "c" => "almaty", "bbox" => [76.6, 43.14, 77.26, 43.5],
        "s" => [[43.25, 76.9, 1], [43.3, 76.95, 7], "junk", [43.2], [43.1, 76.9, 2], [43.26, 76.91, "x"]],
        "me" => [0.5, 0.5]
    };
    var r = MapModel.parse(d);
    Test.assert(r != null);
    Test.assertEqual(r["m"], "map");
    var st = r["st"] as Array;
    // Kept: the first two (the second as gray); dropped: junk, short, outside bbox, non-number category.
    Test.assertEqual(st.size(), 2);
    var a = st[0] as Array;
    Test.assert(near(a[0], 43.25) && near(a[1], 76.9) && a[2] == 1);
    Test.assertEqual((st[1] as Array)[2], MapModel.GRAY);
    var you = r["me"] as Array;
    Test.assert(near(you[0], 43.32) && near(you[1], 76.93));
    var bb = r["bbox"] as Array;
    Test.assert(near(bb[0], 76.6) && near(bb[3], 43.5));
    return true;
}

(:test)
function mapParseWithoutMeOrStations(logger as Logger) as Boolean {
    var r = MapModel.parse({"m" => "near", "bbox" => [76.6, 43.14, 77.26, 43.5], "me" => [2.0, 0.5]});
    Test.assert(r != null);
    Test.assertEqual((r["st"] as Array).size(), 0);
    Test.assert(r["me"] == null);  // out of the unit square
    return true;
}
