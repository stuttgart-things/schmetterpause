"""Source and visible-paint measurements for the unchanged sthings racket."""
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from fontTools.svgLib.path import parse_path
from fontTools.pens.transformPen import TransformPen
from fontTools.misc.transform import Transform
ROOT=Path(__file__).resolve().parents[1]

def shape(d):
 p=pathops.Path();parse_path(d,p.getPen());return p

def moved(p,t):
 o=pathops.Path();p.draw(TransformPen(o.getPen(),t));return o

def union(ps):
 o=pathops.Path()
 for p in ps:o=pathops.op(o,p,pathops.PathOp.UNION)
 return o

def grow(p,d):
 q=pathops.Path(p);q.stroke(2*d,pathops.LineCap.ROUND_CAP,pathops.LineJoin.ROUND_JOIN,4);q.convertConicsToQuads(.001)
 return pathops.op(p,q,pathops.PathOp.UNION)

def gap(a,b):
 lo,hi=0.,150.
 for _ in range(24):
  m=(lo+hi)/2
  if pathops.op(grow(a,m),b,pathops.PathOp.INTERSECTION).area>.001:hi=m
  else:lo=m
 return lo

def reference():
 ps=[]
 for e in ET.parse(ROOT/'web/static/img/sthings/base.svg').getroot().iter():
  if not e.tag.endswith('path'):continue
  p=shape(e.get('d'));p.fillType=pathops.FillType.EVEN_ODD;p.simplify();ps.append((e.get('class'),p))
 blue=next(p for role,p in ps if role=='paddle-face')
 fur=next(p for role,p in ps if role=='fur')
 face=next(c for c in fur.contours if c.bounds[0]==80)
 left=pathops.op(face,shape('M0 0H206V420H0Z'),pathops.PathOp.INTERSECTION)
 reconstructed=union([left,moved(left,Transform(-1,0,0,1,412,0))])
 eyes=[]
 for e in ET.parse(ROOT/'web/static/img/sthings/face-neutral.svg').getroot().iter():
  if e.get('class')!='ink':continue
  p=shape(e.get('d'))
  selected=pathops.Path();selected.fillType=pathops.FillType.EVEN_ODD
  for c in p.contours:
   if c.bounds[0]>=205 and c.bounds[3]<=229:selected.addPath(c)
  selected.simplify();eyes.append(selected)
 visible_handle=pathops.Path();visible_paw=next(c for c in fur.contours if c.bounds[1]>=313)
 for index,(role,p) in enumerate(ps):
  visible_handle=pathops.op(visible_handle,p,pathops.PathOp.DIFFERENCE)
  if role in ('handle','handle-dark'):visible_handle=union([visible_handle,p])
  if index>0:visible_paw=pathops.op(visible_paw,p,pathops.PathOp.DIFFERENCE)
 rim=grow(blue,7)
 return {'blade':blue,'rim':rim,'head_reconstruction':reconstructed,'eye':union(eyes),'paw':visible_paw,'grip':visible_handle,
  'measurements':{'blade_bounds':blue.bounds,'blade_area':blue.area,'width':130,'height':135,'width_percent':130/4.2,'height_percent':135/4.2,
   'centre':[348,228.5],'eyes_y':181.5,'eye_line_offset':47.,'eyes_centre_x':203.75,'face_centre_x':206.,'face_centre_y':195.3,
   'face_overlap_fraction_reconstructed':pathops.op(blue,reconstructed,pathops.PathOp.INTERSECTION).area/blue.area,
   'eye_gap_outline':gap(rim,union(eyes)),'eye_gap_required':max(6,gap(rim,union(eyes))),
   'visible_paw_area':visible_paw.area,'visible_paw_bounds':visible_paw.bounds,'visible_grip_area':visible_handle.area,'visible_grip_bounds':visible_handle.bounds}}
