import Toybox.Lang;
import Toybox.StringUtil;
import Toybox.Test;

(:test)
function heatmapDecodeRoundTrip(logger as Logger) as Boolean {
    var bytes = new [576]b;
    for (var i = 0; i < 576; i++) { bytes[i] = i % 7 == 6 ? 255 : i % 6; }
    var g = StringUtil.convertEncodedString(bytes, {
        :fromRepresentation => StringUtil.REPRESENTATION_BYTE_ARRAY,
        :toRepresentation => StringUtil.REPRESENTATION_STRING_BASE64
    }) as String;
    var r = HeatmapModel.decode(g, 24, 24);
    Test.assert(r != null);
    Test.assertEqual(r.size(), 576);
    for (var i = 0; i < 576; i++) { Test.assertEqual(r[i], bytes[i]); }
    return true;
}

(:test)
function heatmapDecodeWrongLengthIsNull(logger as Logger) as Boolean {
    var bytes = new [575]b;
    var g = StringUtil.convertEncodedString(bytes, {
        :fromRepresentation => StringUtil.REPRESENTATION_BYTE_ARRAY,
        :toRepresentation => StringUtil.REPRESENTATION_STRING_BASE64
    }) as String;
    Test.assert(HeatmapModel.decode(g, 24, 24) == null);
    Test.assert(HeatmapModel.decode("", 24, 24) == null);
    return true;
}

(:test)
function heatmapCellRectGeometry(logger as Logger) as Boolean {
    var r = HeatmapModel.cellRect(0, 0, 24, 24, 168, 46, 46);
    Test.assertEqual(r[0], 46);
    Test.assertEqual(r[1], 46);
    Test.assertEqual(r[2], 7);
    Test.assertEqual(r[3], 7);
    var e = HeatmapModel.cellRect(23, 23, 24, 24, 168, 46, 46);
    Test.assertEqual(e[0] + e[2], 46 + 168);
    Test.assertEqual(e[1] + e[3], 46 + 168);
    return true;
}

(:test)
function heatmapCellRectNoGapsOnUnevenSide(logger as Logger) as Boolean {
    // side 100 / 24 cells does not divide evenly: neighbours must still touch.
    var x = 0;
    for (var i = 0; i < 24; i++) {
        var r = HeatmapModel.cellRect(i, 0, 24, 24, 100, 10, 10);
        Test.assertEqual(r[0], 10 + x);
        Test.assert(r[2] >= 4);
        x += r[2];
    }
    Test.assertEqual(x, 100);
    return true;
}

(:test)
function heatmapToPx(logger as Logger) as Boolean {
    var p = HeatmapModel.toPx([0.5, 0.5], 168, 46, 46);
    Test.assertEqual(p[0], 130);
    Test.assertEqual(p[1], 130);
    return true;
}
