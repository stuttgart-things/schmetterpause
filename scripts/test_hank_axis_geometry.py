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

 def test_binding_and_cap_follow_the_single_shaft(self):
  stripe=capsule_axis(shape(list(self.root)[2].get('d')),.25)['degrees']
  self.assertAlmostEqual(abs(stripe-self.axis['degrees']),90,delta=3)
  self.assertGreater(shape(list(self.root)[3].get('d')).area,0)

 def test_shaft_is_visible_above_and_below_the_mitten(self):
  from hank_flipper_geometry import measurements
  m=measurements()
  self.assertGreaterEqual(m['free_neck_length'],20)
  self.assertGreaterEqual(m['lower_visible_colour_length'],10)
  self.assertLessEqual(abs(m['collinear_deviation_degrees']),2)
  self.assertAlmostEqual(m['grip_axis']['degrees'],13.42,delta=.15)
  self.assertGreater(m['visible_grip_area'],0)

 def test_mitten_proportions_and_full_width_foreground_overlap(self):
  from hank_flipper_geometry import measurements
  m=measurements()
  for key in ('width_ratio','colour_width_ratio'):
   self.assertGreaterEqual(m[key],1.5);self.assertLessEqual(m[key],1.8)
  self.assertGreater(m['full_width_covered_length'],8)
  self.assertEqual(len(self.hand),3,'one front contour, one closed mitten, one fold')
  self.assertEqual(len(list(shape(list(self.hand)[1].get('d')).contours)),1)
  self.assertEqual([g.get('data-pip-part') for g in self.root if g.tag.rsplit('}',1)[-1]=='g'],['flipper','blade'])
  self.assertTrue(all(p.tag.endswith('path') for p in list(self.root)[:5]))

 def test_broad_arm_connects_shoulder_and_mitten_with_clearances(self):
  from hank_flipper_geometry import measurements
  m=measurements()
  self.assertGreater(m['arm_fist_overlap_area'],100)
  self.assertGreaterEqual(m['emblem_clearance'],17.02)
  self.assertGreaterEqual(m['canvas_margin'],13.26)
  self.assertGreaterEqual(m['equipment_canvas_margin'],8-.001)

 def test_excluded_assets_and_blade_are_unchanged(self):
  baseline=json.loads((ROOT/'docs/avatar-check/pip/flippers/invariants-1298bfe.json').read_text())
  for file,expected in baseline.items():
   self.assertEqual(hashlib.sha256((ROOT/file).read_bytes()).hexdigest(),expected,file)
  old=ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-racket.svg').getroot()[0]
  before=next(g for g in old if g.get('data-pip-part')=='blade')
  self.assertEqual([p.attrib for p in before],[p.attrib for p in self.blade])
  old_body=list(ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-base.svg').getroot()[0])
  new_body=list(ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0])
  self.assertEqual(len(old_body),len(new_body))
  changed=[i for i,(a,b) in enumerate(zip(old_body,new_body)) if a.attrib!=b.attrib]
  self.assertEqual(changed,[7,8],'only right-arm outline and colour change; left flipper stays exact')

if __name__=='__main__':unittest.main()
