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

 def test_pressing_flipper_keeps_a_visible_grip_neck(self):
  from hank_flipper_geometry import measurements
  m=measurements()
  self.assertGreaterEqual(m['free_neck_length'],20)
  self.assertGreaterEqual(m['visible_grip_area'],1)
  self.assertGreaterEqual(m['emblem_clearance'],12)
  self.assertGreaterEqual(m['canvas_margin'],8-.001)
  self.assertLess(m['face_overlap_area'],.001)
  self.assertLess(m['blade_overlap_area'],.001)
  self.assertGreaterEqual(m['full_width_covered_fraction'],.30)
  self.assertLessEqual(m['axis_offset_from_overlap_centroid'],5)

 def test_pressing_flipper_is_at_least_75_percent_of_left(self):
  from hank_flipper_geometry import measurements
  m=measurements()
  self.assertGreaterEqual(m['flipper_width_ratio'],.75)
  self.assertLessEqual(m['flipper_width_ratio'],.81)
  self.assertAlmostEqual(m['area_scale'],.8,delta=.001)
  self.assertEqual(len(self.hand),2,'one outline and native folded colour contour, no fingers')
  self.assertEqual(len(list(shape(list(self.hand)[1].get('d')).contours)),1)

 def test_grip_is_behind_flipper_and_ends_wholly_inside_it(self):
  from hank_flipper_geometry import measurements
  m=measurements();a=m['grip_axis'];ux,uy=a['unit'];cx,cy=a['centre']
  self.assertEqual([g.get('data-pip-part') for g in self.root if g.tag.endswith('g')],['flipper','blade'])
  self.assertTrue(all(p.tag.endswith('path') for p in list(self.root)[:4]))
  hand=union(paths(self.hand))
  # Project the actual shortened colour contour to locate its hidden end.
  from hank_frame import transformed
  from fontTools.misc.transform import Transform
  projected=transformed(self.grip,Transform(uy,ux,-ux,uy,0,-cx*ux-cy*uy))
  end=projected.bounds[3]
  for k in range(-17,18):
   self.assertTrue(hand.contains((cx+ux*(end-.2)+uy*k,cy+uy*(end-.2)-ux*k)), 'grip cut must be hidden across its complete width')

 def test_visible_grip_remains_aligned_with_blade(self):
  from hank_flipper_geometry import measurements
  from hank_frame import transformed
  from fontTools.misc.transform import Transform
  m=measurements();a=m['grip_axis'];ux,uy=a['unit'];cx,cy=a['centre']
  import pathops
  from import_creator_avatars import op
  # Whole shaft is symmetric about its baseline axis despite its hidden cut.
  q=transformed(self.grip,Transform(uy,ux,-ux,uy,-cx*uy+cy*ux,0))
  self.assertAlmostEqual(q.bounds[0]+q.bounds[2],0,delta=.15)
  self.assertAlmostEqual(self.axis['degrees'],a['degrees'],delta=.15)

 def test_excluded_assets_and_blade_are_unchanged(self):
  baseline=json.loads((ROOT/'docs/avatar-check/pip/flippers/invariants-1298bfe.json').read_text())
  for file,expected in baseline.items():
   self.assertEqual(hashlib.sha256((ROOT/file).read_bytes()).hexdigest(),expected,file)
  old=ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-racket.svg').getroot()[0]
  before=next(g for g in old if g.get('data-pip-part')=='blade')
  self.assertEqual([p.attrib for p in before],[p.attrib for p in self.blade])
  self.assertEqual((ROOT/'docs/avatar-check/pip/flippers/before-base.svg').read_bytes(),(ROOT/'web/static/img/pip/base.svg').read_bytes(),'1298bfe shoulder and all other body paths stay byte-identical')

if __name__=='__main__':unittest.main()
