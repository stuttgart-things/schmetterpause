"""Approved sthings-style mitten. Final 420-unit coordinates, filled paths only."""
import xml.etree.ElementTree as ET
import pathops
from fontTools.misc.transform import Transform
from import_creator_avatars import shape,ellipse,rect,op,svgpath
from hank_frame import transformed

# Measured original straight shaft, retained verbatim from the approved preview.
AXIS_CENTRE=(327.07724611041976,354.00943704010496)
AXIS_UNIT=(-0.2320971634618589,0.9726926064862214)
RIM=7.
ARM_BODY='M249 317 Q262 311 278 322 L321 348 Q333 357 327 373 Q322 389 307 388 Q291 386 273 373 Q254 361 249 344 Q245 331 249 317Z'
ET.register_namespace('','http://www.w3.org/2000/svg')

def grow(p,distance):
    border=pathops.Path(p)
    border.stroke(2*distance,pathops.LineCap.ROUND_CAP,pathops.LineJoin.ROUND_JOIN,4)
    border.convertConicsToQuads(.001)
    return op(p,border,pathops.PathOp.UNION)

def parts():
    cx,cy=AXIS_CENTRE;ux,uy=AXIS_UNIT
    frame=Transform(uy,-ux,ux,uy,cx,cy)
    fist=transformed(ellipse(0,15.7,22,18),frame)
    pad=transformed(shape('M-18 1 Q-21 7 -19 17 Q-17 28 -8 31 Q0 34 9 30 Q18 26 19 16 Q20 7 16 2 Q8 7 0 7 Q-9 7 -18 1Z'),frame)
    arm=shape(ARM_BODY)
    joined=op(arm,fist,pathops.PathOp.UNION)
    fist_ink=grow(fist,RIM)
    joined_ink=grow(joined,RIM)
    arm_ink=grow(arm,RIM)
    # Outline half-length is 22. The final colour extends another 11.1 units.
    from racket_axis import intervals
    bottom=intervals(fist_ink,(cx-ux*100,cy-uy*100),(ux,uy))[0][1]-100
    end_t=bottom+11-13
    start=(cx-ux*39,cy-uy*39);end=(cx+ux*end_t,cy+uy*end_t)
    shaft=shape(f'M{start[0]} {start[1]} L{end[0]} {end[1]}')
    shaft.stroke(26,pathops.LineCap.ROUND_CAP,pathops.LineJoin.ROUND_JOIN,4)
    shaft.convertConicsToQuads(.001)
    shaft_ink=grow(shaft,5)
    cap=op(shaft,transformed(rect(-30,bottom+8,30,bottom+30),frame),pathops.PathOp.INTERSECTION)
    seam=op(shaft,transformed(rect(-16,bottom+7.5,16,bottom+8.7),frame),pathops.PathOp.INTERSECTION)
    fold=transformed(shape('M-15 15 Q0 7 15 15 Q0 11 -15 15Z'),frame)
    return dict(arm=arm,arm_ink=arm_ink,fist=fist,fist_ink=fist_ink,joined=joined,joined_ink=joined_ink,pad=pad,fold=fold,
                shaft=shaft,shaft_ink=shaft_ink,cap=cap,seam=seam)

def build_assets(base_svg,racket_svg):
    """Replace only the right-arm paths and grip assembly; preserve the blade."""
    p=parts();base=ET.fromstring(base_svg);body=base[0];racket=ET.fromstring(racket_svg)[0]
    # The base retains its original arm. The racket group paints the union of
    # arm and fist over it, so the visible construction has one shared contour.
    direct=[e for e in racket if e.tag.endswith('path')]
    shaft_index=next(i for i,e in enumerate(direct) if e.get('fill','').startswith('var(--handle,'))
    upper=shape(direct[shaft_index+1].get('d'))
    blade=next(g for g in racket if g.get('data-pip-part')=='blade')
    assembly=('<g data-pip-part="joined-arm-fist">'+svgpath(p['joined_ink'],'#080808')+svgpath(p['joined'],'var(--fur, #2f4ea8)')+
              '<g data-pip-part="fist-body">'+svgpath(p['fist'],'var(--fur, #2f4ea8)')+'</g></g>'+
              svgpath(p['shaft_ink'],'#080808')+svgpath(p['shaft'],'var(--handle, #efa31b)')+
              svgpath(upper,'#080808')+svgpath(p['cap'],'var(--handle-dark, #985c10)')+svgpath(p['seam'],'#080808'))
    front=(svgpath(grow(p['pad'],3),'#080808')+svgpath(p['pad'],'var(--fur, #2f4ea8)')+svgpath(p['fold'],'#080808'))
    svg='<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg"><g data-pip-part="racket">'+assembly+'<g data-pip-part="flipper">'+front+'</g>'+ET.tostring(blade,encoding='unicode')+'</g></svg>'
    return ET.tostring(base,encoding='unicode'),svg
