"""Regression: no old racket spur, detached scraps, or unintended geometry edits."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from import_creator_avatars import shape, rect, op
from test_hank_racket_geometry import paths, union

ROOT = Path(__file__).resolve().parents[1]

class HankContourGeometry(unittest.TestCase):
    def test_only_the_fused_cheek_outline_is_changed(self):
        before = ET.parse(ROOT / "docs/avatar-check/pip/cheek-contour/before-base.svg").getroot()
        after = ET.parse(ROOT / "web/static/img/pip/base.svg").getroot()
        for old, new in zip(before, after):
            self.assertEqual(old.attrib, new.attrib)
            if old.get("data-pip-part") != "body":
                self.assertEqual(ET.tostring(old), ET.tostring(new))
            else:
                self.assertEqual([p.attrib for p in old][2:], [p.attrib for p in new][2:])
        difference = op(union(paths(before)), union(paths(after)), pathops.PathOp.XOR)
        self.assertGreater(difference.area, 30)
        self.assertLess(op(difference, rect(318,212,361,241), pathops.PathOp.DIFFERENCE).area, .001)
        silhouette = union(paths(after))
        self.assertLess(op(silhouette, rect(341,212,361,240), pathops.PathOp.INTERSECTION).area, .001)

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
