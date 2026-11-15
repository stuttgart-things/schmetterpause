"""Measurements of B pressing the shortened shaft behind a reflected flipper.
Widths are exact curve extents perpendicular to the measured principal axes.
The baseline capsule supplies the shaft axis, avoiding bias from its hidden cut.
"""
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from fontTools.misc.transform import Transform
from import_creator_avatars import shape,op
from test_hank_racket_geometry import paths,union
from racket_axis import capsule_axis,intervals
from hank_frame import transformed
from measure_hank_pose import clearance
from hank_press import SCALE,ANGLE
ROOT=Path(__file__).resolve().parents[1]

def flippers():
    body=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0]
    left=next(shape(p.get('d')) for p in body if p.get('fill','').startswith('var(--fur,')
              and 52<shape(p.get('d')).bounds[0]<54 and 320<shape(p.get('d')).bounds[1]<324)
    racket=ET.parse(ROOT/'web/static/img/pip/racket.svg').getroot()[0]
    hand=next(g for g in racket if g.get('data-pip-part')=='flipper')
    return left,shape(list(hand)[1].get('d')),hand,racket

def width(p):
    a=capsule_axis(p);ux,uy=a['unit']
    projected=transformed(p,Transform(ux,-uy,uy,ux,0,0))
    _,low,_,high=projected.bounds
    return high-low

def measurements():
    left,right,hand,racket=flippers()
    body=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0]
    grip=next(shape(p.get('d')) for p in racket if p.get('fill','').startswith('var(--handle,'))
    baseline=ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-racket.svg').getroot()[0]
    original=next(shape(p.get('d')) for p in baseline if p.get('fill','').startswith('var(--handle,'))
    axis=capsule_axis(original);ux,uy=axis['unit'];cx,cy=axis['centre'];origin=(cx-ux*100,cy-uy*100)
    blade=next(g for g in racket if g.get('data-pip-part')=='blade')
    stripes=[shape(p.get('d')) for p in list(racket)[2:4]]
    outline=union(paths(hand));visible=op(grip,union(paths(hand)+paths(blade)+stripes),pathops.PathOp.DIFFERENCE)
    emblem=shape(ET.parse(ROOT/'web/static/img/pip/bling-initial.svg').getroot()[0][0].get('d'))
    arm=next(i for i,p in enumerate(body) if p.get('fill','').startswith('var(--fur,') and 234<shape(p.get('d')).bounds[0]<237 and 310<shape(p.get('d')).bounds[1]<320)
    full=union([outline,shape(list(body)[arm-1].get('d'))]);x1,y1,x2,y2=full.bounds
    head=intervals(union(paths(blade)),origin,(ux,uy))[0]
    contact=intervals(outline,origin,(ux,uy))[0]
    face=union([shape(p.get('d')) for p in body if p.get('fill')=='#fff6f3' and shape(p.get('d')).bounds[1]<200])
    return {'reference':'1298bfe','principle':'B: grip behind flipper, blade in foreground','method':__doc__,
        'selected_scale':SCALE,'angle_degrees':ANGLE,'maximum_requested_scale':.8,
        'left_width':width(left),'right_width':width(right),'flipper_width_ratio':width(right)/width(left),
        'area_scale':(right.area/left.area)**.5,
        'emblem_clearance':clearance(emblem,full),'canvas_margin':min(x1,y1,420-x2,420-y2),
        'visible_grip_area':visible.area,'free_neck_length':round(contact[0]-head[1],2),
        'face_overlap_area':op(face,outline,pathops.PathOp.INTERSECTION).area,
        'blade_overlap_area':op(union(paths(blade)),outline,pathops.PathOp.INTERSECTION).area,
        'right_outline_bounds':outline.bounds,'grip_axis':axis}

if __name__=='__main__':
    import json
    print(json.dumps(measurements(),indent=2))
