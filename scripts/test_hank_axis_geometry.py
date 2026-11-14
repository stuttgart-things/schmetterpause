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
  hand_sections=intervals(union(paths(self.hand)),origin,unit)
  hand=(hand_sections[0][0],hand_sections[-1][1])
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
    if 230<x<250 and 280<y<330:arms.append(q)
  from import_creator_avatars import op
  import pathops
  self.assertEqual(len(arms),1)
  self.assertLess(op(arms[0],shape(list(self.hand)[1].get('d')),pathops.PathOp.XOR).area,.01, 'shoulder and front flipper must share one continuous surface')

 def test_rounded_flipper_tip_wraps_the_full_grip_width(self):
  hand=union(paths(self.hand))
  cx,cy=self.axis['centre'];ux,uy=self.axis['unit'];nx,ny=uy,-ux
  covered=[]
  for j in range(0,500):
   t=j*.1;x,y=cx+ux*t,cy+uy*t
   cross=[(x+nx*k*.1,y+ny*k*.1) for k in range(-200,201)
          if self.grip.contains((x+nx*k*.1,y+ny*k*.1))]
   if cross and all(hand.contains(q) for q in cross):covered.append(t)
  self.assertGreater(len(covered),80)
  # One silhouette, one colour surface and only one filled fold: no finger pads.
  self.assertEqual(len(self.hand),3)
  self.assertTrue(all(g.tag.endswith('path') for g in self.hand))
  self.assertEqual(len(list(shape(list(self.hand)[1].get('d')).contours)),1)
  self.assertGreater(max(cx+ux*t for t in covered),hand.bounds[2]-30, 'axis must cross the rounded distal tip')

 def test_both_flipper_stems_have_comparable_width(self):
  from hank_flipper_geometry import measurements
  m=measurements()
  self.assertGreaterEqual(m['minimum_width_ratio'],.85)
  self.assertLessEqual(m['minimum_width_ratio'],1.15)
  self.assertGreaterEqual(m['emblem_clearance'],12)
  self.assertGreaterEqual(m['canvas_margin'],8-.001)
  self.assertGreaterEqual(m['visible_grip_area'],540)

 def test_visible_grip_sections_are_collinear(self):
  import pathops
  from import_creator_avatars import op
  occluders=union(paths(self.hand)+paths(self.blade)+[shape(p.get('d')) for p in list(self.root)[2:4]])
  visible=op(self.grip,occluders,pathops.PathOp.DIFFERENCE)
  origin=self.axis['centre'];ux,uy=self.axis['unit']
  sections=[]
  for t in range(-65,66):
   x,y=origin[0]+ux*t,origin[1]+uy*t
   samples=[k*.1 for k in range(-200,201) if visible.contains((x+uy*k*.1,y-ux*k*.1))]
   grip_samples=[k*.1 for k in range(-200,201) if self.grip.contains((x+uy*k*.1,y-ux*k*.1))]
   # Exclude cross-sections partly clipped by the blade, stripes or hand edge.
   if samples and samples==grip_samples and visible.contains((x,y)):
    self.assertAlmostEqual((min(samples)+max(samples))/2,0,delta=.2)
    sections.append(t)
  self.assertTrue(any(t< -20 for t in sections))
  self.assertTrue(any(t>20 for t in sections))

 def test_excluded_assets_and_blade_are_unchanged(self):
  baseline=json.loads((ROOT/'docs/avatar-check/pip/flippers/invariants-1298bfe.json').read_text())
  for file,expected in baseline.items():
   self.assertEqual(hashlib.sha256((ROOT/file).read_bytes()).hexdigest(),expected,file)
  old=ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-racket.svg').getroot()[0]
  before=next(g for g in old if g.get('data-pip-part')=='blade')
  self.assertEqual([p.attrib for p in before],[p.attrib for p in self.blade])
  old_paths=list(ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-base.svg').getroot()[0])
  new_paths=list(ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0])
  self.assertEqual(len(old_paths),len(new_paths))
  changed=[i for i,(a,b) in enumerate(zip(old_paths,new_paths)) if a.attrib!=b.attrib]
  self.assertEqual(len(changed),2,'only right-flipper outline and fill may change')
  for i in changed:
   self.assertGreater(shape(old_paths[i].get('d')).bounds[0],225)
   self.assertGreater(shape(old_paths[i].get('d')).bounds[1],300)
  old_root=ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-racket.svg').getroot()[0]
  self.assertEqual([p.attrib for p in list(old_root)[:4]],[p.attrib for p in list(self.root)[:4]])

if __name__=='__main__':unittest.main()
