#!/usr/bin/env python3
"""Generate the map marker bitmaps: python3 scripts/gen-markers.py (run from watch/).

marker_0..5.svg: one per AQI category, marker_6.svg: gray (no data). Each is a pixel-exact
12x12 disc with every other pixel transparent (checkerboard): MIP screens have only 1-bit
transparency, so the dither stands in for a semi-transparent dot and the map shows through.
1x1 rects on the pixel grid plus dithering="none" in drawables.xml keep it crisp.
Colors match Aqi.color in source/Aqi.mc.
"""
import os

COLORS = ["00AA00", "FFFF00", "FFAA00", "FF0000", "AA00AA", "550000", "AAAAAA"]
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "resources", "drawables")
SIZE = 12


def checker(color):
    r = SIZE / 2
    rects = "".join(
        f'<rect x="{x}" y="{y}" width="1" height="1"/>'
        for y in range(SIZE)
        for x in range(SIZE)
        if (x + 0.5 - r) ** 2 + (y + 0.5 - r) ** 2 <= r * r and (x + y) % 2 == 0
    )
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{SIZE}" height="{SIZE}" '
            f'viewBox="0 0 {SIZE} {SIZE}" shape-rendering="crispEdges"><g fill="#{color}">{rects}</g></svg>\n')


for i, c in enumerate(COLORS):
    with open(os.path.join(OUT, f"marker_{i}.svg"), "w") as f:
        f.write(checker(c))
