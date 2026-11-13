#!/usr/bin/env python3
"""Compare the current hank holding pose to the untouched sthings source."""
import json
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape,svgpath,op
from test_hank_racket_geometry import union,paths,grow
from measure_hank_pose import clearance,measurements
from sthings_racket_reference import reference
from hank_frame import SCALE,OFFSET_X,OFFSET_Y
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'docs/avatar-check/pip/holding';ASSETS=ROOT/'web/static/img/pip'

def export():
 OUT.mkdir(exist_ok=True);ref=reference();base=ET.parse(ASSETS/'base.svg').getroot()[0]
 racket=ET.parse(ASSETS/'racket.svg').getroot()[0]
 blade=next(g for g in racket if g.get('data-pip-part')=='blade');hand=next(g for g in racket if g.get('data-pip-part')=='flipper')
 surface=shape(list(blade)[-1].get('d'));rim=union(paths(blade));paw=union(paths(hand))
 visible_paw=op(paw,rim,pathops.PathOp.DIFFERENCE)
 colour=pathops.Path()
 for p in hand:
  q=shape(p.get('d'));colour=op(colour,q,pathops.PathOp.DIFFERENCE)
  if p.get('fill')!='#080808':colour=op(colour,q,pathops.PathOp.UNION)
 colour=op(colour,rim,pathops.PathOp.DIFFERENCE)
 face=next(shape(e.get('d')) for e in base if e.get('fill')=='#fff6f3' and shape(e.get('d')).bounds[1]<200)
 neutral=ET.parse(ASSETS/'face-neutral.svg').getroot()[0]
 eyes=[union(paths(g)) for g in neutral if g.get('data-pip-part','').startswith('eye-')]
 eye_y=sum((p.bounds[1]+p.bounds[3])/2 for p in eyes)/2
 eye_x=sum((p.bounds[0]+p.bounds[2])/2 for p in eyes)/2
 x1,y1,x2,y2=surface.bounds;cx,cy=(x1+x2)/2,(y1+y2)/2
 pose=measurements();emblem=shape(ET.parse(ASSETS/'bling-initial.svg').getroot()[0][0].get('d'))
 beak_gaps={}
 for state in ('neutral','happy','surprised','skeptical','angry','sad'):
  beak=next(g for g in ET.parse(ASSETS/('face-'+state+'.svg')).getroot().iter() if g.get('data-pip-part')=='beak')
  beak_gaps[state]=clearance(emblem,union(paths(beak)))
 shaft=next(shape(e.get('d')) for e in racket if e.get('fill','').startswith('var(--handle,'))
 stripes=[shape(e.get('d')) for e in list(racket)[2:4]]
 grip=op(shaft,union(paths(hand)+paths(blade)+stripes),pathops.PathOp.DIFFERENCE)
 data={'sthings':ref['measurements'],'hank':{
  'frame_scale':SCALE,'frame_offset':[OFFSET_X,OFFSET_Y],'width':x2-x1,'height':y2-y1,'width_percent':(x2-x1)/4.2,'height_percent':(y2-y1)/4.2,
  'blade_centre':[cx,cy],'eyes_y':eye_y,'eyes_centre_x':eye_x,'eye_line_offset':cy-eye_y,
  'face_centre':[(face.bounds[0]+face.bounds[2])/2,(face.bounds[1]+face.bounds[3])/2],
  'face_overlap_fraction':op(surface,face,pathops.PathOp.INTERSECTION).area/surface.area,
  'hand_bounds':paw.bounds,'hand_visible_fraction':visible_paw.area/paw.area,'visible_hand_colour_area':colour.area,'visible_hand_colour_bounds':colour.bounds,
  'visible_grip_area':grip.area,'visible_grip_bounds':grip.bounds,'canvas_margin':pose['canvas_margin'],
  'feature_clearances':pose['feature_clearances'],'emblem_size':pose['emblem_size'],'emblem_bottom_margin':pose['emblem_bottom_margin'],
  'emblem_beak_clearance':beak_gaps,'emblem_racket_clearance':pose['emblem_racket_clearance'],'emblem_flipper_clearance':pose['emblem_flipper_clearance'],
  'native_bottom_crop':(420-OFFSET_Y)/SCALE,'chin_to_bottom':420-(315*SCALE+OFFSET_Y)},
  'notes':{'face_overlap':'sthings is reconstructed by mirroring the visible left face at x=206; its native face is already cut around the paddle. hank uses its complete white face surface.',
  'reference_outline':'7-unit original outer rim, evidenced by the blue/black right canvas bounds 413/420; hank uses at least max(reference gap,6).',
  'visible_areas':'Paint-order-aware selected grip and paw surfaces; black hand outline is additionally used for full-hand visibility and canvas tests.'}}
 (OUT/'measurements.json').write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n')
 masks={'sthings-blade':ref['blade'],'sthings-paw':ref['paw'],'sthings-grip':ref['grip'],'hank-blade':surface,'hank-paw':colour,'hank-grip':grip}
 for key,p in masks.items():
  (OUT/(key+'-mask.svg')).write_text('<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg">'+svgpath(p,'#ffffff')+'</svg>\n')
 return data

if __name__=='__main__':
 d=export()
 print(json.dumps(d,indent=2,ensure_ascii=False))
