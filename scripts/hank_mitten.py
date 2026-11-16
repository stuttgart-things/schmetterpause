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
    fist=transformed(ellipse(0,15.7,22,15),frame)
    fist_ink=grow(fist,RIM);arm=shape(ARM_BODY);arm_ink=grow(arm,RIM)
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
    front_ink=op(fist_ink,arm,pathops.PathOp.DIFFERENCE)
    fold=transformed(shape('M-16 13 Q-1 6 15 13 Q-1 9 -16 13Z'),frame)
    return dict(arm=arm,arm_ink=arm_ink,fist=fist,fist_ink=fist_ink,front_ink=front_ink,fold=fold,
                shaft=shaft,shaft_ink=shaft_ink,cap=cap,seam=seam)

def build_assets(base_svg,racket_svg):
    """Replace only the right-arm paths and grip assembly; preserve the blade."""
    p=parts();base=ET.fromstring(base_svg);body=base[0];racket=ET.fromstring(racket_svg)[0]
    i=next(i for i,e in enumerate(body) if e.get('fill','').startswith('var(--fur,')
           and 234<shape(e.get('d')).bounds[0]<237 and 310<shape(e.get('d')).bounds[1]<320)
    for element,geometry,fill in ((body[i-1],p['arm_ink'],'#080808'),(body[i],p['arm'],'var(--fur, #2f4ea8)')):
        attrs=ET.fromstring('<svg>'+svgpath(geometry,fill)+'</svg>')[0].attrib
        element.attrib.clear();element.attrib.update(attrs)
    upper=shape(list(racket)[2].get('d'))
    blade=next(g for g in racket if g.get('data-pip-part')=='blade')
    assembly=(svgpath(p['shaft_ink'],'#080808')+svgpath(p['shaft'],'var(--handle, #efa31b)')+
              svgpath(upper,'#080808')+svgpath(p['cap'],'var(--handle-dark, #985c10)')+svgpath(p['seam'],'#080808'))
    front=(svgpath(p['front_ink'],'#080808')+svgpath(p['fist'],'var(--fur, #2f4ea8)')+svgpath(p['fold'],'#080808'))
    svg='<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg"><g data-pip-part="racket">'+assembly+'<g data-pip-part="flipper">'+front+'</g>'+ET.tostring(blade,encoding='unicode')+'</g></svg>'
    return ET.tostring(base,encoding='unicode'),svg
