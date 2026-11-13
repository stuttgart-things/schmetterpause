"""New framing constraints plus a fully round foreground racket."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from hank_frame import SCALE,CREST_HEIGHT_FACTOR,framed
from import_creator_avatars import shape,ellipse,op
from test_hank_racket_geometry import paths,union
ROOT=Path(__file__).resolve().parents[1]

class HankFramingGeometry(unittest.TestCase):
    def test_crest_keeps_top_margin_and_shortening_limit(self):
        base=ET.parse(ROOT/"web/static/img/pip/base.svg").getroot()
        self.assertGreater(SCALE,1)
        self.assertGreaterEqual(CREST_HEIGHT_FACTOR,.6)
        self.assertGreaterEqual(union(paths(base)).bounds[1],8-.001)

    def test_complete_round_blade_follows_the_gripping_flipper(self):
        root=ET.parse(ROOT/"web/static/img/pip/racket.svg").getroot()[0]
        names=[g.get("data-pip-part") for g in root if g.tag.endswith("g")]
        self.assertEqual(names,["flipper","blade"])
        blade=list(root)[-1]
        colour=shape(list(blade)[-1].get("d"))
        self.assertLess(op(colour,framed(ellipse(317,290,42.3,43)),pathops.PathOp.XOR).area,.1)
        before=ET.parse(ROOT/"docs/avatar-check/pip/framing/before-base.svg").getroot()
        old=next(shape(e.get("d")) for e in before.iter() if e.get("fill", "").startswith("var(--paddle,"))
        x1,y1,x2,y2=colour.bounds;ox1,oy1,ox2,oy2=old.bounds
        self.assertGreaterEqual(x2-x1,(ox2-ox1)*SCALE*.8-.001)
        self.assertGreaterEqual(y2-y1,(oy2-oy1)*SCALE*.8-.001)

if __name__=="__main__":unittest.main()
