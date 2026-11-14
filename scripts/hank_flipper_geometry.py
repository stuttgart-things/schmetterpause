"""Measure rendered flipper stems perpendicular to their own long axes.

The comparable stem excludes the shoulder junction (first quarter) and the
rounded distal cap (last 35%). Sample every 0.5% through the remaining stem.
Envelope widths span the fold notch, rather than mistaking it for a thin arm.
"""
import xml.etree.ElementTree as ET
from pathlib import Path
from racket_axis import capsule_axis, intervals
from import_creator_avatars import shape, op
from test_hank_racket_geometry import paths, union
from measure_hank_pose import clearance
from hank_pose import BLADE_MASS_CENTRE
from hank_frame import transformed
from fontTools.misc.transform import Transform
import pathops

ROOT = Path(__file__).resolve().parents[1]

def flippers():
    body=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0]
    left=next(shape(p.get('d')) for p in body if p.get('fill','').startswith('var(--fur,')
              and 52<shape(p.get('d')).bounds[0]<54 and 320<shape(p.get('d')).bounds[1]<324)
    racket=ET.parse(ROOT/'web/static/img/pip/racket.svg').getroot()[0]
    hand=next(g for g in racket if g.get('data-pip-part')=='flipper')
    right=shape(list(hand)[1].get('d'))
    return left,right,hand,racket

def stem_widths(p):
    axis=capsule_axis(p);cx,cy=axis['centre'];ux,uy=axis['unit']
    # Exact curve extrema in axis space; Bezier control points are not edges.
    projected=transformed(p,Transform(ux,-uy,uy,ux,-cx*ux-cy*uy,0))
    lo,_,hi,_=projected.bounds
    result=[]
    for j in range(50,131):
        f=j/200;t=lo+(hi-lo)*f;x,y=cx+ux*t,cy+uy*t
        sections=intervals(p,(x-uy*120,y+ux*120),(uy,-ux),limit=240)
        result.append({'fraction':f,'width':round(sections[-1][1]-sections[0][0],2)})
    return result

def measurements():
    left,right,hand,racket=flippers()
    lw,rw=stem_widths(left),stem_widths(right)
    grip=next(shape(p.get('d')) for p in racket if p.get('fill','').startswith('var(--handle,'))
    blade=next(g for g in racket if g.get('data-pip-part')=='blade')
    stripes=[shape(p.get('d')) for p in list(racket)[2:4]]
    visible=op(grip,union(paths(hand)+paths(blade)+stripes),pathops.PathOp.DIFFERENCE)
    emblem=shape(ET.parse(ROOT/'web/static/img/pip/bling-initial.svg').getroot()[0][0].get('d'))
    outline=union(paths(hand));x1,y1,x2,y2=outline.bounds
    unit=capsule_axis(grip)['unit'];origin=BLADE_MASS_CENTRE
    head_interval=intervals(union(paths(blade)),origin,unit)[0]
    hand_intervals=intervals(outline,origin,unit)
    grip_interval=intervals(grip,origin,unit)[0]
    return {'reference':'1298bfe','method':__doc__,'left_stem_min':min(r['width'] for r in lw),
            'right_stem_min':min(r['width'] for r in rw),
            'minimum_width_ratio':min(r['width'] for r in rw)/min(r['width'] for r in lw),
            'left_sections':lw,'right_sections':rw,'emblem_clearance':clearance(emblem,outline),
            'canvas_margin':min(x1,y1,420-x2,420-y2),'visible_grip_area':visible.area,
            'free_neck_length':hand_intervals[0][0]-head_interval[1],
            'lower_grip_length':grip_interval[1]-hand_intervals[-1][1],
            'right_outline_bounds':outline.bounds,'grip_axis':capsule_axis(grip)}

if __name__=='__main__':
    import json
    print(json.dumps(measurements(),indent=2))
