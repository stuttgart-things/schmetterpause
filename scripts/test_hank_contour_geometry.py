"""Regression: no old racket spur, detached scraps, or unintended geometry edits."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from import_creator_avatars import shape, rect, op
from test_hank_racket_geometry import paths, union
from hank_frame import framed

ROOT = Path(__file__).resolve().parents[1]

class HankContourGeometry(unittest.TestCase):
    def test_body_changes_only_by_uniform_frame_and_permitted_crest_shortening(self):
        before=ET.parse(ROOT/"docs/avatar-check/pip/framing/before-base.svg").getroot()
        after=ET.parse(ROOT/"web/static/img/pip/base.svg").getroot()
        expected=union([framed(p,crest=True,crop=True) for p in paths(before[0])])
        self.assertLess(op(expected,union(paths(after[0])),pathops.PathOp.XOR).area,.1)
        self.assertLess(op(expected,framed(rect(341,212,361,240)),pathops.PathOp.INTERSECTION).area,.001)

    def test_white_cheek_has_no_seam_between_native_and_rebuilt_surfaces(self):
        after=ET.parse(ROOT/"web/static/img/pip/base.svg").getroot()
        white=union([shape(el.get("d")) for el in after[0] if el.get("fill")=="#fff6f3"])
        self.assertLess(op(framed(rect(280,205,320,212)),white,pathops.PathOp.DIFFERENCE).area,.001)

    def test_no_unassigned_small_path_fragments(self):
        base=ET.parse(ROOT/"web/static/img/pip/base.svg").getroot()
        equipment=ET.parse(ROOT/"web/static/img/pip/racket.svg").getroot()
        self.assertEqual([g.get("data-pip-part") for g in base],["body"])
        allpaths=paths(base)+paths(equipment)
        large=union([p for p in allpaths if p.area>=30])
        before=ET.parse(ROOT/"docs/avatar-check/pip/framing/before-base.svg").getroot()
        shades=union([framed(shape(p.get("d")),crest=True,crop=True) for p in before[0] if p.get("fill","").startswith("var(--fur-dark,")])
        allowed=op(large,shades,pathops.PathOp.UNION)
        for p in allpaths:
            for contour in p.contours:
                if contour.area<30:self.assertLess(op(contour,allowed,pathops.PathOp.DIFFERENCE).area,.001,contour.bounds)

if __name__ == "__main__":
    unittest.main()
