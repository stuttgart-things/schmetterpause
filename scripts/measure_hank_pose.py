#!/usr/bin/env python3
"""Measure the current filled-path pose, not a raster approximation."""
import json
import re
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape, op, rect, ellipse
from test_hank_racket_geometry import paths, union, grow
from hank_frame import SCALE,OFFSET_X,OFFSET_Y

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
    parts['racket']=ET.parse(ASSETS/'racket.svg').getroot()[0]
    parts['flipper']=next(g for g in parts['racket'] if g.get('data-pip-part')=='flipper')
    racket=union(paths(parts['racket']))
    flipper=union(paths(parts['flipper']))
    blade=next(g for g in parts['racket'] if g.get('data-pip-part')=='blade')
    grip=next(shape(p.get('d')) for p in parts['racket'] if p.get('fill','').startswith('var(--handle,'))
    stripes=[shape(p.get('d')) for p in list(parts['racket'])[2:4]]
    visible=op(grip,union(paths(blade)+paths(parts['flipper'])+stripes),pathops.PathOp.DIFFERENCE)
    head=op(union(paths(parts['body'])),rect(0,0,420,315*SCALE+OFFSET_Y),pathops.PathOp.INTERSECTION)
    overlap=op(head,union(paths(blade)),pathops.PathOp.INTERSECTION)
    y=285*SCALE+OFFSET_Y
    strip=op(overlap,rect(0,y-.01,420,y+.01),pathops.PathOp.INTERSECTION)
    features={}
    for state in ('neutral','happy','surprised','skeptical','angry','sad'):
        face=ET.parse(ASSETS/('face-'+state+'.svg')).getroot()
        features[state]={g.get('data-pip-part'):clearance(racket,union(paths(g))) for g in face[0]}
    emblem=shape(ET.parse(ASSETS/"bling-initial.svg").getroot()[0][0].get("d"))
    bounds=[p.bounds for child in parts["racket"] if child.get("data-pip-part")!="flipper" for p in ([shape(child.get("d"))] if child.tag.endswith("path") else paths(child))]
    return {'eye_reference':'sthings actual contours','eye_scale':.8*SCALE,
        'frame':{'scale':SCALE,'offset':[OFFSET_X,OFFSET_Y]},
        'racket_shape':'complete sthings paddle contour',
        'emblem_size':emblem.bounds[2]-emblem.bounds[0],
        'emblem_bottom_margin':420-emblem.bounds[3],
        'racket_path_bounds':bounds,'canvas_margin':round(min(min(x1,y1,420-x2,420-y2) for x1,y1,x2,y2 in bounds),3),
        'cheek_overlap_area':round(overlap.area,3),'cheek_overlap_width_at_native_y285':round(strip.bounds[2]-strip.bounds[0],3),
        'feature_clearances':features,'emblem_racket_clearance':clearance(emblem,racket),
        'emblem_flipper_clearance':clearance(emblem,flipper),
        'visible_grip_colour_area':round(visible.area,3),'visible_grip_colour_bounds':visible.bounds}

if __name__=='__main__':
    data=measurements()
    for target in (ROOT/'docs/avatar-check/pip/straight/pose-measurements.json',ROOT/'docs/avatar-check/pip/racket-measurements.json'):
        target.parent.mkdir(parents=True,exist_ok=True)
        target.write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n')
    emblem=shape(ET.parse(ASSETS/'bling-initial.svg').getroot()[0][0].get('d'))
    x1,y1,x2,y2=emblem.bounds
    arms=[]
    for el in ET.parse(ASSETS/'base.svg').getroot()[0]:
        if el.get('fill','').startswith('var(--fur,'):
            q=shape(el.get('d'));ax,ay,_,_=q.bounds;ax=(ax-OFFSET_X)/SCALE;ay=(ay-OFFSET_Y)/SCALE
            if (60<ax<80 and 315<ay<320) or (230<ax<240 and 280<ay<320):arms.append(grow(q,5.5*SCALE))
    eq=ET.parse(ASSETS/'racket.svg').getroot()[0]
    hand=next(g for g in eq if g.get('data-pip-part')=='flipper')
    letters=[]
    for letter,encoded in re.findall(r'^\s*"([A-ZÄÖÜ])":\s*("(?:\\.|[^"\\])*"),',(ROOT/'internal/avatar/hank_glyphs.go').read_text(),re.M):
        glyph=ET.fromstring('<svg>'+json.loads(encoded)+'</svg>')
        gx1,gy1,gx2,gy2=shape(list(glyph)[-1].get('d')).bounds
        letters.append({'initial':letter,'width':round(gx2-gx1,3),'height':round(gy2-gy1,3),'height_at_64':round((gy2-gy1)*64/420,2)})
    emblem_data={'emblem_size':x2-x1,'centre':[(x1+x2)/2,(y1+y2)/2],
        'bottom_margin':data['emblem_bottom_margin'],'flipper_clearance':clearance(emblem,union(arms+paths(hand))),
        'racket_clearance':data['emblem_racket_clearance'],'visible_selected_grip_colour_area':data['visible_grip_colour_area'],'initials':letters}
    (ROOT/'docs/avatar-check/pip/emblems/measurements.json').write_text(json.dumps(emblem_data,indent=2,ensure_ascii=False)+'\n')
    print(json.dumps(data,indent=2,ensure_ascii=False))
