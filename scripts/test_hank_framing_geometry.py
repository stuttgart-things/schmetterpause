"""New framing constraints plus a fully round foreground racket."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from hank_frame import SCALE,CREST_HEIGHT_FACTOR,framed
from sthings_racket_reference import reference,moved
from hank_pose import BLADE_SCALE,BLADE_CENTRE
from fontTools.misc.transform import Transform
from import_creator_avatars import shape,ellipse,op
from test_hank_racket_geometry import paths,union
ROOT=Path(__file__).resolve().parents[1]

class HankFramingGeometry(unittest.TestCase):
    def test_crest_keeps_top_margin_and_shortening_limit(self):
        base=ET.parse(ROOT/"web/static/img/pip/base.svg").getroot()
        self.assertGreaterEqual(SCALE,1.10)
        self.assertLessEqual(SCALE,1.15)
        self.assertGreaterEqual(CREST_HEIGHT_FACTOR,.6)
        self.assertGreaterEqual(union(paths(base)).bounds[1],8-.001)

    def test_complete_round_blade_follows_the_gripping_flipper(self):
        root=ET.parse(ROOT/"web/static/img/pip/racket.svg").getroot()[0]
        names=[g.get("data-pip-part") for g in root if g.tag.endswith("g")]
        self.assertEqual(names,["flipper","blade"])
        blade=list(root)[-1]
        colour=shape(list(blade)[-1].get("d"))
        ref=reference()['blade']
        expected=moved(ref,Transform(BLADE_SCALE,0,0,BLADE_SCALE,BLADE_CENTRE[0]-348*BLADE_SCALE,BLADE_CENTRE[1]-228.5*BLADE_SCALE))
        self.assertLess(op(colour,expected,pathops.PathOp.XOR).area,.1)
        self.assertEqual(len(list(colour.contours)),1)

if __name__=="__main__":unittest.main()
