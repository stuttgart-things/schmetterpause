"""Contact coverage measured from an SVG's actual shaft and flipper paths."""
import math
import xml.etree.ElementTree as ET
import pathops
from fontTools.misc.transform import Transform
from hank_frame import transformed
from racket_axis import capsule_axis
from import_creator_avatars import shape,op
from test_hank_racket_geometry import paths,union

def contact_metrics(svg,axis=None):
 root=ET.fromstring(svg)
 racket=next(g for g in root.iter() if g.get('data-pip-part')=='racket')
 hand=union(paths(next(g for g in racket if g.get('data-pip-part')=='flipper')))
 shaft=shape(list(racket)[0].get('d'))
 axis=axis or capsule_axis(shaft);cx,cy=axis['centre'];ux,uy=axis['unit']
 q=transformed(shaft,Transform(uy,ux,-ux,uy,-cx*uy+cy*ux,-cx*ux-cy*uy))
 lo,hi=q.bounds[1],q.bounds[3];full=[]
 for j in range(math.ceil(lo*4),math.floor(hi*4)+1):
  t=j/4;x,y=cx+ux*t,cy+uy*t
  cross=[(x+uy*k/4,y-ux*k/4) for k in range(-80,81) if shaft.contains((x+uy*k/4,y-ux*k/4))]
  if cross and all(hand.contains(p) for p in cross):full.append(t)
 overlap=op(shaft,hand,pathops.PathOp.INTERSECTION)
 x1,y1,x2,y2=overlap.bounds;count=sx=sy=0
 for j in range(math.ceil(y1*4),math.floor(y2*4)+1):
  for k in range(math.ceil(x1*4),math.floor(x2*4)+1):
   x,y=k/4,j/4
   if overlap.contains((x,y)):sx+=x;sy+=y;count+=1
 point=(sx/count,sy/count)
 return {'shaft_length':hi-lo,'full_width_covered_length':max(full)-min(full),'full_width_covered_fraction':(max(full)-min(full))/(hi-lo),
         'overlap_centroid':point,'axis_offset_from_overlap_centroid':abs((point[0]-cx)*uy-(point[1]-cy)*ux),
         'axis':axis,'full_width_interval':[min(full),max(full)]}
