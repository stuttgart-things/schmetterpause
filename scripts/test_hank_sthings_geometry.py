"""Artifact checks against sthings, including the original even-odd pupil cut-outs."""
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from fontTools.misc.transform import Transform
from fontTools.pens.transformPen import TransformPen
from import_creator_avatars import shape, op
from test_hank_racket_geometry import union, paths, grow
from hank_frame import framed

ROOT=Path(__file__).resolve().parents[1]/"web/static/img"

class HankSthingsGeometry(unittest.TestCase):
    def test_eye_contours_are_the_scaled_sthings_reference(self):
        for state in ("neutral","happy","surprised","skeptical","angry","sad"):
            source=ET.parse(ROOT/"sthings"/("face-"+state+".svg")).getroot()
            target=ET.parse(ROOT/"pip"/("face-"+state+".svg")).getroot()
            for index,(sx,cx,cy) in enumerate(((161,145,236),(246,255,214))):
                eye=next(g for g in target.iter() if g.get("data-pip-part")=="eye-"+str(index))
                expected=[]
                for el in source.iter("{http://www.w3.org/2000/svg}path"):
                    p=shape(el.get("d"));result=pathops.Path();result.fillType=pathops.FillType.EVEN_ODD
                    for contour in p.contours:
                        x1,y1,x2,y2=contour.bounds
                        if y2<=229 and ((index==0 and x2<=203) or (index==1 and x1>=205)):
                            result.addPath(contour)
                    if not list(result.contours):continue
                    result.simplify()
                    transformed=pathops.Path()
                    result.draw(TransformPen(transformed.getPen(),Transform(.8,0,0,.8,cx-sx*.8,cy-181*.8)))
                    expected.append((el.get("fill"),framed(transformed)))
                self.assertEqual(len(eye),len(expected))
                for actual,(fill,p) in zip(eye,expected):
                    self.assertEqual(actual.get("fill"),fill)
                    self.assertLess(op(shape(actual.get("d")),p,pathops.PathOp.XOR).area,.1,(state,index))

    def test_other_five_beaks_only_receive_the_uniform_frame(self):
        for state in ("neutral","happy","surprised","skeptical","sad"):
            before=ET.parse(ROOT.parents[2]/"docs/avatar-check/pip/framing"/("before-face-"+state+".svg")).getroot()
            after=ET.parse(ROOT/"pip"/("face-"+state+".svg")).getroot()
            old=next(g for g in before.iter() if g.get("data-pip-part")=="beak")
            new=next(g for g in after.iter() if g.get("data-pip-part")=="beak")
            self.assertEqual(len(old),len(new))
            for a,b in zip(old,new):
                self.assertEqual(a.get("fill"),b.get("fill"))
                self.assertLess(op(framed(shape(a.get("d"))),shape(b.get("d")),pathops.PathOp.XOR).area,.1,state)

    def test_eye_artwork_keeps_eight_units_from_unchanged_headband(self):
        base=ET.parse(ROOT/"pip/base.svg").getroot()
        band=union([shape(e.get("d")) for e in base[0] if e.get("fill", "").startswith("var(--headdress")])
        for state in ("neutral","happy","surprised","skeptical","angry","sad"):
            face=ET.parse(ROOT/"pip"/("face-"+state+".svg")).getroot()
            for eye in [g for g in face.iter() if g.get("data-pip-part", "").startswith("eye-")]:
                self.assertLess(op(grow(union(paths(eye)),8),band,pathops.PathOp.INTERSECTION).area,.001,state)

if __name__=="__main__":unittest.main()
