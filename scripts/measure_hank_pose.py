#!/usr/bin/env python3
"""Measure the current filled-path pose, not a raster approximation."""
import json
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape, op, rect, ellipse
from test_hank_racket_geometry import paths, union, grow

ROOT=Path(__file__).resolve().parents[1]
ASSETS=ROOT/'web/static/img/pip'

def clearance(a,b):
    lo,hi=0.,150.
    for _ in range(24):
        mid=(lo+hi)/2
        if op(grow(a,mid),b,pathops.PathOp.INTERSECTION).area>.001:hi=mid
        else:lo=mid
    return round(lo,3)

def measurements():
    root=ET.parse(ASSETS/'base.svg').getroot()
    parts={g.get('data-pip-part'):g for g in root}
    racket=union(paths(parts['racket']))
    flipper=union(paths(parts['flipper']))
    blade=next(g for g in parts['racket'] if g.get('data-pip-part')=='blade')
    grip=next(shape(p.get('d')) for p in parts['racket'] if p.get('fill','').startswith('var(--handle,'))
    stripes=[shape(p.get('d')) for p in list(parts['racket'])[2:4]]
    visible=op(grip,union(paths(blade)+paths(parts['flipper'])+stripes),pathops.PathOp.DIFFERENCE)
    head=op(union(paths(parts['body'])),rect(0,0,420,315),pathops.PathOp.INTERSECTION)
    overlap=op(head,union(paths(blade)),pathops.PathOp.INTERSECTION)
    strip=op(overlap,rect(0,244.99,420,245.01),pathops.PathOp.INTERSECTION)
    features={}
    for state in ('neutral','happy','surprised','skeptical','angry','sad'):
        face=ET.parse(ASSETS/('face-'+state+'.svg')).getroot()
        features[state]={g.get('data-pip-part'):clearance(racket,union(paths(g))) for g in face[0]}
    emblem=ellipse(209.8,366.1,28.5,28.5)
    bounds=[p.bounds for p in paths(parts['racket'])]
    return {'eye_reference':'sthings actual contours','eye_scale':.8,
        'rotation_degrees':18,'head_scale':.9,'head_offset_after_rotation':[-8,-52],
        'grip_offset_from_native':[8,-80],'flipper_offset_from_previous':[15,-70],
        'racket_path_bounds':bounds,'canvas_margin':round(min(min(x1,y1,420-x2,420-y2) for x1,y1,x2,y2 in bounds),3),
        'cheek_overlap_area':round(overlap.area,3),'cheek_overlap_width_at_y245':round(strip.bounds[2]-strip.bounds[0],3),
        'feature_clearances':features,'emblem_racket_clearance':clearance(emblem,racket),
        'emblem_flipper_clearance':clearance(emblem,flipper),
        'visible_grip_colour_area':round(visible.area,3),'visible_grip_colour_bounds':visible.bounds}

if __name__=='__main__':
    data=measurements()
    for target in (ROOT/'docs/avatar-check/pip/sthings-style/measurements.json',ROOT/'docs/avatar-check/pip/racket-measurements.json'):
        target.parent.mkdir(parents=True,exist_ok=True)
        target.write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n')
    print(json.dumps(data,indent=2,ensure_ascii=False))
