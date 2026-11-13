#!/usr/bin/env python3
"""Record the functional and rim-normal axes and actual visible shaft intervals."""
import json,math
from pathlib import Path
import xml.etree.ElementTree as ET
from racket_axis import reference_axis,capsule_axis,bottom_normal,intervals
from sthings_racket_reference import reference,grow
from import_creator_avatars import shape,op
from test_hank_racket_geometry import union,paths
from measure_hank_holding import export
from hank_pose import BLADE_MASS_CENTRE
import pathops
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'docs/avatar-check/pip/straight'

def summary(root):
 blade=next(g for g in root if g.get('data-pip-part')=='blade');hand=next(g for g in root if g.get('data-pip-part')=='flipper')
 surface=shape(list(blade)[-1].get('d'));grip=next(shape(p.get('d')) for p in root if p.get('fill','').startswith('var(--handle,'))
 axis=capsule_axis(grip);unit=axis['unit']
 cx,cy=axis['centre'];hx,hy=BLADE_MASS_CENTRE
 t=(hx-cx)*unit[0]+(hy-cy)*unit[1]
 origin=(cx+t*unit[0],cy+t*unit[1])
 offset=abs((hx-cx)*unit[1]-(hy-cy)*unit[0])
 head=intervals(union(paths(blade)),origin,unit)[0];fist=intervals(union(paths(hand)),origin,unit)[0];shaft=intervals(grip,origin,unit)[0]
 stripes=union([shape(p.get('d')) for p in list(root)[2:4]])
 visible=op(grip,union(paths(hand)+paths(blade)+[stripes]),pathops.PathOp.DIFFERENCE)
 visible_intervals=intervals(visible,origin,unit)
 return {'grip_axis_degrees':axis['degrees'],'head_centre_to_grip_axis':offset,'blade_rim_normal_degrees':bottom_normal(surface),
  'axis_difference_degrees':abs(axis['degrees']-bottom_normal(surface)),
  'blade_functional_axis_degrees':reference_axis()['blade_axis_degrees'],
  'functional_axis_difference_degrees':abs(axis['degrees']-reference_axis()['blade_axis_degrees']),
  'hand_bounds':union(paths(hand)).bounds,'blade_bounds':surface.bounds,'visible_grip_area':visible.area,
  'neck_length':fist[0]-head[1],'lower_grip_end_length':shaft[1]-fist[1],
  'blade_axis_interval':head,'hand_axis_interval':fist,'grip_axis_interval':shaft,'visible_colour_intervals':visible_intervals}

def main():
 export(OUT)
 ref=reference();a=reference_axis();t=math.radians(a['blade_axis_degrees']);u=(-math.sin(t),math.cos(t));o=a['blade_centre']
 rim=intervals(ref['rim'],o,u)[0];paw=intervals(grow(ref['paw'],7),o,u)[0]
 visible=intervals(ref['grip'],o,u)
 a.update({'blade_rim_normal_degrees':bottom_normal(ref['blade']),'paw_bounds':ref['paw'].bounds,
  'paw_bbox_centre_offset':[((ref['paw'].bounds[0]+ref['paw'].bounds[2])/2)-o[0],((ref['paw'].bounds[1]+ref['paw'].bounds[3])/2)-o[1]],
  'visible_grip_intervals':visible,'upper_visible_grip_length':visible[0][1]-visible[0][0],
  'lower_visible_grip_length':visible[-1][1]-visible[-1][0],'outline_neck_length':paw[0]-rim[1],
  'method':'Blade centre = area centroid; functional axis goes toward the upper dominant grip field. Handle axis uses the two dominant visible grip-field centroids. A lower-rim normal gives an independent geometric blade direction. Paw outline is reconstructed with the original 7-unit rim.'})
 current=ET.parse(ROOT/'web/static/img/pip/racket.svg').getroot()[0]
 before=ET.parse(OUT/'before-racket.svg').getroot()[0]
 result={'sthings':a,'hank_before':summary(before),'hank_after':summary(current),
  'shaft_measurement':'Principal axis of points sampled inside the actual complete filled capsule, 0.5-unit grid; no intended endpoint constants are used for its angle.',
  'interval_measurement':'Lengths along the handle centreline, between the blade outline, hand outline and colour end; stripes interrupt colour but not the neck span.'}
 (OUT/'axis-measurements.json').write_text(json.dumps(result,indent=2,ensure_ascii=False)+'\n')
 print(json.dumps(result,indent=2,ensure_ascii=False))

if __name__=='__main__':main()
