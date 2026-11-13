"""Exact filled-path checks for the final emblem and the isolated thumb change."""

import json
import re
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape, ellipse, op, creator_svg, rect
from test_hank_racket_geometry import paths, union, grow
from hank_frame import framed,SCALE,OFFSET_X,OFFSET_Y,EMBLEM

ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / "web/static/img/pip"


class HankEmblemGeometry(unittest.TestCase):
    def setUp(self):
        self.root = ET.parse(ASSETS / "base.svg").getroot()
        self.g = {g.get("data-pip-part"): g for g in self.root}
        self.g["racket"]=ET.parse(ASSETS/"racket.svg").getroot()[0]
        self.g["flipper"]=next(g for g in self.g["racket"] if g.get("data-pip-part")=="flipper")
        self.emblem = shape(
            ET.parse(ASSETS / "bling-initial.svg").getroot()[0][0].get("d")
        )

    def test_emblem_fits_belly_with_all_clearances(self):
        belly = next(
            shape(el.get("d"))
            for el in self.g["body"]
            if el.get("fill") == "#fff6f3"
            and shape(el.get("d")).bounds[1] >= 314*SCALE+OFFSET_Y
            and (shape(el.get("d")).bounds[0]-OFFSET_X)/SCALE > 145
        )
        # The torso is intentionally cut by the canvas; the bottom clearance
        # is 8 units, while the anatomical belly sides keep their 10-unit inset.
        old=ET.parse(ROOT/"docs/avatar-check/pip/framing/before-base.svg").getroot()
        native=next(shape(e.get("d")) for e in old[0] if e.get("fill")=="#fff6f3" and shape(e.get("d")).bounds[1]>=314 and shape(e.get("d")).bounds[0]>145)
        belly=framed(native)
        self.assertLessEqual(self.emblem.bounds[3],412)
        border = pathops.Path(belly)
        border.stroke(20, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4)
        border.convertConicsToQuads(0.001)
        safe = op(belly, border, pathops.PathOp.DIFFERENCE)
        self.assertLess(op(self.emblem, safe, pathops.PathOp.DIFFERENCE).area, 0.001)
        body = paths(self.g["body"])
        # The unchanged non-racket arm paths are the native blue surfaces
        # entering from the left and upper-right side of the belly.
        arms = []
        for el in self.g["body"]:
            if el.get("fill", "").startswith("var(--fur,"):
                p = shape(el.get("d"))
                x1, y1, x2, y2 = p.bounds
                x1=(x1-OFFSET_X)/SCALE; y1=(y1-OFFSET_Y)/SCALE
                if (60 < x1 < 80 and 315 < y1 < 320) or (
                    230 < x1 < 240 and 280 < y1 < 320
                ):
                    arms.append(p)
        self.assertEqual(len(arms), 2)
        flippers = union([grow(p,5.5*SCALE) for p in arms] + paths(self.g["flipper"]))
        self.assertLess(
            op(self.emblem, grow(flippers, 12), pathops.PathOp.INTERSECTION).area, 0.001
        )
        self.assertLess(
            op(
                self.emblem,
                grow(union(paths(self.g["racket"])), 12),
                pathops.PathOp.INTERSECTION,
            ).area,
            0.001,
        )
        for key in ["neutral", "happy", "surprised", "skeptical", "angry", "sad"]:
            face = ET.parse(ASSETS / ("face-" + key + ".svg")).getroot()[0]
            beak = next(g for g in face if g.get("data-pip-part") == "beak")
            self.assertLess(
                op(
                    self.emblem,
                    grow(union(paths(beak)), 10),
                    pathops.PathOp.INTERSECTION,
                ).area,
                0.001,
            )
        x1, y1, x2, y2 = self.emblem.bounds
        self.assertAlmostEqual(x2 - x1, 48, delta=0.002)
        self.assertAlmostEqual(y2 - y1, 48, delta=0.002)

    def test_every_glyph_outline_stays_within_shield(self):
        text = (ROOT / "internal/avatar/hank_glyphs.go").read_text()
        entries = re.findall(r'^\s*"([A-ZÄÖÜ])":\s*("(?:\\.|[^"\\])*"),', text, re.M)
        self.assertEqual(len(entries), 29)
        for letter, encoded in entries:
            for el in ET.fromstring("<svg>" + json.loads(encoded) + "</svg>"):
                self.assertLess(
                    op(shape(el.get("d")), self.emblem, pathops.PathOp.DIFFERENCE).area,
                    0.001,
                    letter,
                )

    def test_fallback_is_one_solid_contour_and_fits_letter_inset(self):
        root = ET.parse(ASSETS / "bling-initial.svg").getroot()
        fallback = next(g for g in root.iter() if g.get("data-initial") == "fallback")
        self.assertEqual(len(fallback), 2)
        cx,cy,r=EMBLEM
        interior = framed(ellipse(cx,cy,r-6.5,r-6.5))
        for el in fallback:
            d = el.get("d")
            self.assertEqual(len(re.findall(r"[Mm]", d)), 1, "inner subpath/hole")
            self.assertLess(op(shape(d), interior, pathops.PathOp.DIFFERENCE).area, .001)
        self.assertLess(shape(fallback[-1].get("d")).area, self.emblem.area * .6)

    def test_raised_thumb_still_wraps_grip_and_flipper(self):
        thumb = shape(list(self.g["flipper"])[-1].get("d"))
        paw = shape(list(self.g["flipper"])[1].get("d"))
        grip = next(shape(el.get("d")) for el in self.g["racket"] if el.get("fill", "").startswith("var(--handle,"))
        self.assertGreater(op(thumb,grip,pathops.PathOp.INTERSECTION).area,10)
        self.assertGreater(op(thumb,paw,pathops.PathOp.INTERSECTION).area,10)


if __name__ == "__main__":
    unittest.main()
