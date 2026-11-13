import Toybox.Lang;
import Toybox.StringUtil;

// Pure geometry and decoding for the heatmap screen (full widget process only).
module HeatmapModel {
    // Base64 grid -> bytes, or null when the length is not w*h or the text is not valid base64.
    function decode(g as String, w as Number, h as Number) as ByteArray? {
        try {
            var b = StringUtil.convertEncodedString(g, {
                :fromRepresentation => StringUtil.REPRESENTATION_STRING_BASE64,
                :toRepresentation => StringUtil.REPRESENTATION_BYTE_ARRAY
            }) as ByteArray?;
            return (b != null && b.size() == w * h) ? b : null;
        } catch (ex) {
            return null;
        }
    }

    // Pixel rectangle [x, y, cw, ch] of column i / row j; neighbours share edges (no gaps).
    function cellRect(i as Number, j as Number, w as Number, h as Number, side as Number, ox as Number, oy as Number) as Array<Number> {
        var x = ox + (i * side) / w;
        var y = oy + (j * side) / h;
        var x2 = ox + ((i + 1) * side) / w;
        var y2 = oy + ((j + 1) * side) / h;
        return [x, y, x2 - x, y2 - y];
    }

    // Fraction pair [fx, fy] (0..1 of the map square) -> screen pixel [x, y].
    function toPx(frac as Array, side as Number, ox as Number, oy as Number) as Array<Number> {
        var fx = frac[0] as Number or Float or Double;
        var fy = frac[1] as Number or Float or Double;
        // Fractions are non-negative here, so +0.5 and truncation rounds half up.
        return [ox + (fx * side + 0.5).toNumber(), oy + (fy * side + 0.5).toNumber()];
    }
}
