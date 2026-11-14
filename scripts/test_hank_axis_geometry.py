"""Rigid geometry measured from generated paths, not just intended constants."""
import unittest,json,hashlib,math
import xml.etree.ElementTree as ET
from pathlib import Path
from racket_axis import reference_axis,capsule_axis,bottom_normal,intervals
from hank_pose import BLADE_MASS_CENTRE
from import_creator_avatars import shape
from test_hank_racket_geometry import paths,union
ROOT=Path(__file__).resolve().parents[1]

class HankAxisGeometry(unittest.TestCase):
 def setUp(self):
  self.root=ET.parse(ROOT/'web/static/img/pip/racket.svg').getroot()[0]
  self.blade=next(g for g in self.root if g.get('data-pip-part')=='blade')
  self.hand=next(g for g in self.root if g.get('data-pip-part')=='flipper')
  self.grip=next(shape(p.get('d')) for p in self.root if p.get('fill','').startswith('var(--handle,'))
  self.axis=capsule_axis(self.grip)

 def test_face_and_handle_are_one_outward_rigid_axis(self):
  angle=self.axis['degrees'];head=bottom_normal(shape(list(self.blade)[-1].get('d')))
  self.assertLessEqual(abs(angle-head),3)
  self.assertLessEqual(abs(angle-reference_axis()['whole_axis_degrees']),5)
  self.assertGreaterEqual(angle,0)
  cx,cy=self.axis['centre'];ux,uy=self.axis['unit'];hx,hy=BLADE_MASS_CENTRE
  self.assertLess(abs((hx-cx)*uy-(hy-cy)*ux),.5)
  self.assertGreater(cy,hy)

 def test_stripes_are_perpendicular_to_the_shaft(self):
  for p in list(self.root)[2:4]:
   stripe=capsule_axis(shape(p.get('d')),.25)['degrees']
   self.assertAlmostEqual(abs(stripe-self.axis['degrees']),90,delta=3)

 def test_visible_neck_and_short_lower_end(self):
  origin=BLADE_MASS_CENTRE;unit=self.axis['unit']
  head=intervals(union(paths(self.blade)),origin,unit)[0]
  hand=intervals(union(paths(self.hand)),origin,unit)[0]
  grip=intervals(self.grip,origin,unit)[0]
  neck=hand[0]-head[1]
  self.assertGreaterEqual(neck,20);self.assertLessEqual(neck,30)
  self.assertGreater(grip[1]-hand[1],3)
  body=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0]
  from hank_frame import SCALE,OFFSET_X,OFFSET_Y
  arms=[]
  for p in body:
   if p.get('fill','').startswith('var(--fur,'):
    q=shape(p.get('d'));x,y,_,_=q.bounds;x=(x-OFFSET_X)/SCALE;y=(y-OFFSET_Y)/SCALE
    if 230<x<240 and 280<y<320:arms.append(q)
  from import_creator_avatars import op
  import pathops
  self.assertGreater(op(union(arms),union(paths(self.hand)),pathops.PathOp.INTERSECTION).area,15)

 def test_excluded_assets_and_blade_are_unchanged(self):
  baseline=json.loads((ROOT/'docs/avatar-check/pip/straight/invariants-8822413.json').read_text())
  for file,expected in baseline.items():
   if "face-" in file: continue # Beaks intentionally redesigned; eyes checked separately.
   self.assertEqual(hashlib.sha256((ROOT/file).read_bytes()).hexdigest(),expected,file)
  old=ET.parse(ROOT/'docs/avatar-check/pip/straight/before-racket.svg').getroot()[0]
  before=next(g for g in old if g.get('data-pip-part')=='blade')
  self.assertEqual([p.attrib for p in before],[p.attrib for p in self.blade])

if __name__=='__main__':unittest.main()
