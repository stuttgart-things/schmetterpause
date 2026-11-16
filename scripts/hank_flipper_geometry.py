"""Independent measurements of the approved mitten from exported SVG paths."""
import math
import xml.etree.ElementTree as ET
from pathlib import Path
import pathops
from fontTools.misc.transform import Transform
from import_creator_avatars import shape,op
from test_hank_racket_geometry import paths,union,grow
from racket_axis import capsule_axis,intervals
from hank_frame import transformed
from measure_hank_pose import clearance
ROOT=Path(__file__).resolve().parents[1]

def flippers():
    body=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0]
    left=next(shape(p.get('d')) for p in body if p.get('fill','').startswith('var(--fur,') and 52<shape(p.get('d')).bounds[0]<54 and 320<shape(p.get('d')).bounds[1]<324)
    racket=ET.parse(ROOT/'web/static/img/pip/racket.svg').getroot()[0]
    hand=next(g for g in racket if g.get('data-pip-part')=='flipper')
    return left,shape(list(hand)[1].get('d')),hand,racket

def measurements():
    left,fist,hand,racket=flippers()
    body=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot()[0]
    shaft=shape(list(racket)[1].get('d'));shaft_ink=shape(list(racket)[0].get('d'))
    # This complete capsule supplies the axis, not intended endpoint constants.
    axis=capsule_axis(shaft);cx,cy=axis['centre'];ux,uy=axis['unit'];nx,ny=uy,-ux
    frame=Transform(nx,ux,ny,uy,-cx*nx-cy*ny,-cx*ux-cy*uy)
    projected=lambda p:transformed(p,frame)
    fist_ink=grow(fist,7)
    blade=next(g for g in racket if g.get('data-pip-part')=='blade')
    arm_index=next(i for i,p in enumerate(body) if p.get('fill','').startswith('var(--fur,') and 246<shape(p.get('d')).bounds[0]<250 and shape(p.get('d')).bounds[1]>310)
    arm=shape(list(body)[arm_index].get('d'));arm_ink=shape(list(body)[arm_index-1].get('d'))
    emblem=shape(ET.parse(ROOT/'web/static/img/pip/bling-initial.svg').getroot()[0][0].get('d'))
    assembly=union([arm_ink,fist_ink,shaft_ink]);equipment=union([assembly]+paths(blade))
    def margin(p):x1,y1,x2,y2=p.bounds;return min(x1,y1,420-x2,420-y2)
    origin=(cx-ux*150,cy-uy*150)
    hi=intervals(union(paths(blade)),origin,(ux,uy),limit=300)[0]
    fi=intervals(fist_ink,origin,(ux,uy),limit=300)[0]
    si=intervals(shaft,origin,(ux,uy),limit=300)[0]
    # Cross-section centres in the two revealed portions are measured separately.
    centres=[]
    for t in ((hi[1]+fi[0])/2,(fi[1]+si[1])/2):
        x,y=origin[0]+ux*t,origin[1]+uy*t
        spans=intervals(shaft,(x-nx*60,y-ny*60),(nx,ny),limit=120)
        n=(spans[0][0]+spans[-1][1])/2-60
        centres.append((t,n))
    delta=math.degrees(math.atan2(centres[1][1]-centres[0][1],centres[1][0]-centres[0][0]))
    full=[];front_mask=union(paths(hand))
    for k in range(int(fi[0]*4),int(fi[1]*4)+1):
        t=k/4;x,y=origin[0]+ux*t,origin[1]+uy*t
        cross=[(x+nx*j/4,y+ny*j/4) for j in range(-80,81) if shaft_ink.contains((x+nx*j/4,y+ny*j/4))]
        if cross and all(front_mask.contains(q) for q in cross):full.append(t)
    visible=op(shaft,union(paths(hand)+paths(blade)+[shape(p.get('d')) for p in list(racket)[2:5]]),pathops.PathOp.DIFFERENCE)
    fw=projected(fist).bounds[2]-projected(fist).bounds[0];sw=projected(shaft).bounds[2]-projected(shaft).bounds[0]
    ow=projected(fist_ink).bounds[2]-projected(fist_ink).bounds[0];iw=projected(shaft_ink).bounds[2]-projected(shaft_ink).bounds[0]
    return {'principle':'mitten grips a continuous capped shaft','reference':'approved mitten preview; a99eac2 blade and face geometry',
            'grip_axis':axis,'fist_colour_width':fw,'shaft_colour_width':sw,'colour_width_ratio':fw/sw,
            'fist_outline_width':ow,'shaft_outline_width':iw,'width_ratio':ow/iw,
            'free_neck_length':round(fi[0]-hi[1],2),'lower_visible_colour_length':round(si[1]-fi[1],2),
            'collinear_deviation_degrees':delta,'full_width_covered_length':max(full)-min(full),
            'arm_fist_overlap_area':op(arm,fist,pathops.PathOp.INTERSECTION).area,
            'emblem_clearance':clearance(emblem,assembly),'canvas_margin':margin(assembly),'equipment_canvas_margin':margin(equipment),
            'visible_grip_area':visible.area,'cap_area':shape(list(racket)[3].get('d')).area,
            'hand_axis_interval':fi,'shaft_axis_interval':si,'neck_and_lower_centres':centres}

if __name__=='__main__':
    import json
    print(json.dumps(measurements(),indent=2))
