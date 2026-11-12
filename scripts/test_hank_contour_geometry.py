"""Regression: no old racket spur, detached scraps, or unintended geometry edits."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from import_creator_avatars import shape, rect, op
from test_hank_racket_geometry import paths, union

ROOT = Path(__file__).resolve().parents[1]

class HankContourGeometry(unittest.TestCase):
    def test_head_outline_has_no_old_spur_and_only_authorized_body_regions_change(self):
        before = ET.parse(ROOT / "docs/avatar-check/pip/sthings-style/before-base.svg").getroot()
        after = ET.parse(ROOT / "web/static/img/pip/base.svg").getroot()
        self.assertEqual(before[0][0].attrib,after[0][0].attrib)
        self.assertEqual(before[0][1].attrib,after[0][1].attrib)
        changed = op(union(paths(before[0])),union(paths(after[0])),pathops.PathOp.XOR)
        allowed = union([rect(230,280,350,420),rect(245,198,341,310)])
        self.assertLess(op(changed,allowed,pathops.PathOp.DIFFERENCE).area,.001)
        # Inspect the bare head, not the racket now deliberately covering it.
        head=op(union(paths(after[0])),rect(0,0,420,280),pathops.PathOp.INTERSECTION)
        self.assertLess(op(head,rect(341,212,361,240),pathops.PathOp.INTERSECTION).area,.001)
        oldband=[el.attrib for el in before[0] if el.get("fill", "").startswith("var(--headdress")]
        newband=[el.attrib for el in after[0] if el.get("fill", "").startswith("var(--headdress")]
        self.assertEqual(oldband,newband)

    def test_white_cheek_has_no_seam_between_native_and_rebuilt_surfaces(self):
        after=ET.parse(ROOT / "web/static/img/pip/base.svg").getroot()
        white=union([shape(el.get("d")) for el in after[0] if el.get("fill")=="#fff6f3"])
        self.assertLess(op(rect(280,205,320,212),white,pathops.PathOp.DIFFERENCE).area,.001)

    def test_no_unassigned_small_path_fragments(self):
        before = ET.parse(ROOT / "docs/avatar-check/pip/cheek-contour/before-base.svg").getroot()
        native_shadows = {p.get("d") for p in before[0] if p.get("fill", "").startswith("var(--fur-dark,")}
        after = ET.parse(ROOT / "web/static/img/pip/base.svg").getroot()
        self.assertEqual([g.get("data-pip-part") for g in after], ["body", "racket", "flipper"])
        large = union([p for p in paths(after) if p.area >= 30])
        for g in after:
            for el in g.iter("{http://www.w3.org/2000/svg}path"):
                # Original coat shading is assigned to body and is intentional.
                if g.get("data-pip-part") == "body" and el.get("d") in native_shadows:
                    continue
                for contour in shape(el.get("d")).contours:
                    if contour.area < 30:
                        self.assertLess(op(contour, large, pathops.PathOp.DIFFERENCE).area, .001, (g.attrib, contour.bounds))

if __name__ == "__main__":
    unittest.main()
