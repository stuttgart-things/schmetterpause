"""Build a private review proposal; never writes the production SVG assets."""
import sys, xml.etree.ElementTree as ET, pathops
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
from import_creator_avatars import shape,ellipse,op,svgpath
from hank_mitten import grow, AXIS_CENTRE, AXIS_UNIT, ARM_BODY
from hank_frame import transformed
from fontTools.misc.transform import Transform

OUT=ROOT/'docs/avatar-check/pip/full-fist-preview'
ET.register_namespace('','http://www.w3.org/2000/svg')

def proposal():
    cx,cy=AXIS_CENTRE; ux,uy=AXIS_UNIT
    frame=Transform(uy,-ux,ux,uy,cx,cy)
    # Sthings-inspired full palm: broad across the shaft and deep enough to read as a fist.
    body=transformed(ellipse(0,15.7,22,18),frame)
    pad=transformed(shape('M-18 1 Q-21 7 -19 17 Q-17 28 -8 31 Q0 34 9 30 Q18 26 19 16 Q20 7 16 2 Q8 7 0 7 Q-9 7 -18 1Z'),frame)
    arm=shape(ARM_BODY)
    joined=op(arm,body,pathops.PathOp.UNION)
    joined_ink=grow(joined,7)
    return body,pad,joined,joined_ink,frame

def build():
    base=ET.parse(ROOT/'web/static/img/pip/base.svg').getroot(); racket=ET.parse(ROOT/'web/static/img/pip/racket.svg').getroot()[0]
    body,pad,joined,joined_ink,frame=proposal()
    b=base[0]; i=next(i for i,e in enumerate(b) if e.get('fill','').startswith('var(--fur,') and 246<shape(e.get('d')).bounds[0]<250 and shape(e.get('d')).bounds[1]>310 and shape(e.get('d')).bounds[3]<400)
    # Remove old arm paint; the joined arm/palm will be part of the racket group,
    # where it can remain behind the shaft while retaining one uninterrupted outline.
    # Keep base untouched; the racket group paints this combined silhouette over it.
    # Keep current handle geometry and blade unchanged; paint full palm behind shaft,
    # then a broad domed finger side over the shaft.
    direct=[e for e in racket if e.tag.endswith('path')]
    shaft_i=next(i for i,e in enumerate(direct) if e.get('fill','').startswith('var(--handle,'))
    shaft=shape(direct[shaft_i].get('d')); shaft_ink=shape(direct[shaft_i-1].get('d'))
    blade=next(g for g in racket if g.get('data-pip-part')=='blade')
    fold=transformed(shape('M-15 15 Q0 7 15 15 Q0 11 -15 15Z'),frame)
    markup=('<g data-pip-part="joined-arm-fist">'+svgpath(joined_ink,'#080808')+svgpath(joined,'var(--fur, #2f4ea8)')+
            '<g data-pip-part="fist-body">'+svgpath(body,'var(--fur, #2f4ea8)')+'</g></g>'+ 
            svgpath(shaft_ink,'#080808')+svgpath(shaft,'var(--handle, #efa31b)')+
            ''.join(ET.tostring(e,encoding='unicode') for e in direct[shaft_i+1:shaft_i+4])+ 
            '<g data-pip-part="flipper">'+svgpath(pad,'var(--fur, #2f4ea8)')+svgpath(fold,'#080808')+'</g>'+ET.tostring(blade,encoding='unicode'))
    rs=ET.fromstring('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 420 420"><g data-pip-part="racket">'+markup+'</g></svg>')
    OUT.mkdir(parents=True,exist_ok=True)
    (OUT/'proposal-base.svg').write_text(ET.tostring(base,encoding='unicode'))
    (OUT/'proposal-racket.svg').write_text(ET.tostring(rs,encoding='unicode'))

if __name__=='__main__':build()
