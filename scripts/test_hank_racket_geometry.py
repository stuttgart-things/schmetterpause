"""Exact curve-space bounds and clearance checks for Hank's baked SVG paths."""

import unittest
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape, op, rect, ellipse
from hank_frame import SCALE,OFFSET_Y

ROOT = Path(__file__).resolve().parents[1] / "web/static/img/pip"


def union(paths):
    result = pathops.Path()
    for p in paths:
        result = op(result, p, pathops.PathOp.UNION)
    return result


def grow(p, distance):
    border = pathops.Path(p)
    border.stroke(
        distance * 2, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4
    )
    border.convertConicsToQuads(0.001)
    return op(p, border, pathops.PathOp.UNION)


def paths(group):
    return [shape(el.get("d")) for el in group.iter("{http://www.w3.org/2000/svg}path")]


class HankRacketGeometry(unittest.TestCase):
    def setUp(self):
        root = ET.parse(ROOT / "base.svg").getroot()
        self.parts = {g.get("data-pip-part"): g for g in root}
        self.racket=ET.parse(ROOT/"racket.svg").getroot()[0]
        self.parts["racket"]=self.racket
        self.parts["flipper"]=next(g for g in self.racket if g.get("data-pip-part")=="flipper")

    def test_every_racket_path_including_ink_has_eight_unit_canvas_margin(self):
        for index, p in enumerate([p for child in self.racket if child.get("data-pip-part")!="flipper" for p in ([shape(child.get("d"))] if child.tag.endswith("path") else paths(child))]):
            with self.subTest(path=index):
                x1, y1, x2, y2 = p.bounds
                self.assertGreaterEqual(min(x1, y1), 8 - 0.001)
                self.assertLessEqual(max(x2, y2), 412 + 0.001)

    def test_racket_overlaps_cheek_and_clears_every_face_feature(self):
        head = op(union(paths(self.parts["body"])), rect(0,0,420,315*SCALE+OFFSET_Y), pathops.PathOp.INTERSECTION)
        racket = union(paths(self.racket))
        for face in ("neutral","happy","surprised","skeptical","angry","sad"):
            features=ET.parse(ROOT / ("face-"+face+".svg")).getroot()
            for p in paths(features):
                self.assertLess(op(grow(p,12),racket,pathops.PathOp.INTERSECTION).area,.001,face)
        blade = next(g for g in self.racket if g.get("data-pip-part")=="blade")
        overlap=op(head,union(paths(blade)),pathops.PathOp.INTERSECTION)
        y=285*SCALE+OFFSET_Y
        strip=op(overlap,rect(0,y-.01,420,y+.01),pathops.PathOp.INTERSECTION)
        self.assertGreaterEqual(strip.bounds[2]-strip.bounds[0],20)
        self.assertGreater(overlap.area,400)
        reserve=shape(ET.parse(ROOT/"bling-initial.svg").getroot()[0][0].get("d"))
        for name in ("racket","flipper"):
            self.assertLess(op(grow(reserve,12),union(paths(self.parts[name])),pathops.PathOp.INTERSECTION).area,.001,name)

    def test_selected_grip_colour_keeps_at_least_thirty_by_eighteen_equivalent_area(
        self,
    ):
        grip = next(
            shape(el.get("d"))
            for el in self.racket
            if el.get("fill", "").startswith("var(--handle,")
        )
        blade = next(g for g in self.racket if g.get("data-pip-part") == "blade")
        stripes = [shape(el.get("d")) for el in list(self.racket)[2:4]]
        cover = union(paths(blade) + paths(self.parts["flipper"]) + stripes)
        visible = op(grip, cover, pathops.PathOp.DIFFERENCE)
        x1, y1, x2, y2 = visible.bounds
        self.assertGreaterEqual(visible.area, 30 * 18)
        self.assertGreaterEqual(x2 - x1, 30)
        self.assertGreaterEqual(y2 - y1, 18)


if __name__ == "__main__":
    unittest.main()
