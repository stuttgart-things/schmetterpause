"""Functional paddle/handle axes; near-round blade PCA is not a useful shaft axis."""
import math
import xml.etree.ElementTree as ET
from pathlib import Path
from sthings_racket_reference import reference,shape
ROOT=Path(__file__).resolve().parents[1]

def polygon_centre(p):
 points=list(p.points);a=x=y=0.
 for (u,v),(s,t) in zip(points,points[1:]+points[:1]):
  c=u*t-s*v;a+=c;x+=(u+s)*c;y+=(v+t)*c
 return x/(3*a),y/(3*a)

def tilt(a,b):
 return math.degrees(math.atan2(a[0]-b[0],b[1]-a[1]))

def reference_axis():
 ref=reference();blue=ref['blade'];centre=polygon_centre(blue)
 handle=next(shape(e.get('d')) for e in ET.parse(ROOT/'web/static/img/sthings/base.svg').getroot().iter() if e.get('class')=='handle')
 top=polygon_centre(next(c for c in handle.contours if c.bounds[0]==322))
 end=polygon_centre(next(c for c in handle.contours if c.bounds[1]>=383))
 return {'blade_centre':centre,'upper_grip_centre':top,'lower_grip_centre':end,
  'blade_axis_degrees':tilt(centre,top),'grip_axis_degrees':tilt(top,end),'whole_axis_degrees':tilt(centre,end)}

def capsule_axis(p,step=.5):
 x1,y1,x2,y2=p.bounds;points=[]
 y=y1+step/2
 while y<y2:
  x=x1+step/2
  while x<x2:
   if p.contains((x,y)):points.append((x,y))
   x+=step
  y+=step
 cx=sum(x for x,y in points)/len(points);cy=sum(y for x,y in points)/len(points)
 xx=sum((x-cx)**2 for x,y in points);yy=sum((y-cy)**2 for x,y in points);xy=sum((x-cx)*(y-cy) for x,y in points)
 t=.5*math.atan2(2*xy,xx-yy);vx,vy=math.cos(t),math.sin(t)
 if vy<0:vx,vy=-vx,-vy
 return {'centre':(cx,cy),'unit':(vx,vy),'degrees':math.degrees(math.atan2(-vx,vy))}

def intervals(mask,origin,unit,limit=220,step=.1):
 result=[];start=None
 for i in range(int(limit/step)+1):
  t=i*step;inside=mask.contains((origin[0]+unit[0]*t,origin[1]+unit[1]*t))
  if inside and start is None:start=t
  if not inside and start is not None:
   if t-start>.5:result.append((round(start,2),round(t,2)))
   start=None
 return result

def bottom_normal(p):
 points=list(p.points);_,y1,_,y2=p.bounds;centre_y=(y1+y2)/2
 candidates=[]
 for a,b in zip(points,points[1:]+points[:1]):
  if min(a[1],b[1])>centre_y+.25*(y2-y1):
   dx,dy=b[0]-a[0],b[1]-a[1];candidates.append((math.hypot(dx,dy),dx,dy))
 _,dx,dy=max(candidates);vx,vy=-dy,dx
 if vy<0:vx,vy=-vx,-vy
 return math.degrees(math.atan2(-vx,vy))
