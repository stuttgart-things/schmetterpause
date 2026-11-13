"""Exact curve-space bounds and clearance checks for Hank's baked SVG paths."""

import unittest
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape, op, rect, ellipse
from hank_frame import SCALE,OFFSET_Y
from sthings_racket_reference import reference

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

    def test_reference_size_position_overlap_and_feature_clearance(self):
        ref=reference()['measurements']
        blade=next(g for g in self.racket if g.get("data-pip-part")=="blade")
        surface=shape(list(blade)[-1].get("d"))
        rim=union(paths(blade));x1,y1,x2,y2=surface.bounds
        self.assertGreaterEqual(x2-x1,ref['width']*.9-.001)
        self.assertLessEqual(x2-x1,ref['width']*1.1+.001)
        self.assertGreaterEqual(y2-y1,ref['height']*.9-.001)
        self.assertLessEqual(y2-y1,ref['height']*1.1+.001)
        neutral=ET.parse(ROOT/'face-neutral.svg').getroot()
        eyeline=sum((union(paths(g)).bounds[1]+union(paths(g)).bounds[3])/2 for g in neutral[0] if g.get('data-pip-part','').startswith('eye-'))/2
        delta=(y1+y2)/2-eyeline
        self.assertAlmostEqual(delta,ref['eye_line_offset'],delta=ref['eye_line_offset']*.1)
        face=next(shape(e.get('d')) for e in self.parts['body'] if e.get('fill')=='#fff6f3' and shape(e.get('d')).bounds[1]<200)
        overlap=op(surface,face,pathops.PathOp.INTERSECTION).area/surface.area
        self.assertAlmostEqual(overlap,ref['face_overlap_fraction_reconstructed'],delta=ref['face_overlap_fraction_reconstructed']*.1)
        for state in ('neutral','happy','surprised','skeptical','angry','sad'):
            root=ET.parse(ROOT/('face-'+state+'.svg')).getroot()
            for g in root[0]:
                margin=8 if g.get('data-pip-part')=='beak' else ref['eye_gap_required']
                self.assertLess(op(grow(union(paths(g)),margin),rim,pathops.PathOp.INTERSECTION).area,.001,(state,g.attrib))
        reserve=shape(ET.parse(ROOT/'bling-initial.svg').getroot()[0][0].get('d'))
        self.assertLess(op(grow(reserve,12),union(paths(self.racket)),pathops.PathOp.INTERSECTION).area,.001)

    def test_entire_hand_is_inside_canvas_and_visible_below_blade(self):
        hand=union(paths(self.parts['flipper']))
        x1,y1,x2,y2=hand.bounds
        self.assertGreaterEqual(min(x1,y1),8-.001)
        self.assertLessEqual(max(x2,y2),412+.001)
        blade=next(g for g in self.racket if g.get('data-pip-part')=='blade')
        visible=op(hand,union(paths(blade)),pathops.PathOp.DIFFERENCE)
        self.assertGreaterEqual(visible.area/hand.area,.95)

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
