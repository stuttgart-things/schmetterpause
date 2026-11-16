#!/usr/bin/env python3
"""Archived approved mitten preview based on a99eac2. Never writes product assets."""
import hashlib,json,math,subprocess,sys,xml.etree.ElementTree as ET
from pathlib import Path
ROOT=Path(__file__).resolve().parents[4];OUT=Path(__file__).resolve().parent
sys.path.insert(0,str(ROOT/'scripts'))
from import_creator_avatars import shape,op,svgpath,ellipse,rect
from test_hank_racket_geometry import paths,union,grow
from hank_frame import transformed
from fontTools.misc.transform import Transform
from racket_axis import capsule_axis,intervals,reference_axis
from sthings_racket_reference import reference
from measure_hank_pose import clearance
import pathops
ET.register_namespace('','http://www.w3.org/2000/svg')

def archive(part):return subprocess.check_output(['git','show','a99eac2:web/static/img/pip/'+part+'.svg'],cwd=ROOT)
def digest():return {str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'web/static/img').rglob('*.svg')}
def generate():
 before=digest();head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
 base=ET.fromstring(archive('base'));racket=ET.fromstring(archive('racket'))[0]
 original=ET.parse(ROOT/'docs/avatar-check/pip/flippers/before-racket.svg').getroot()[0]
 shaft0=next(shape(p.get('d')) for p in original if p.get('fill','').startswith('var(--handle,'))
 axis=capsule_axis(shaft0);cx,cy=axis['centre'];ux,uy=axis['unit'];nx,ny=uy,-ux
 frame=Transform(nx,ny,ux,uy,cx,cy)
 # Widths including outline: 58-unit round mitten / 36-unit shaft = 1.611.
 # Colour widths: 44 / 26 = 1.692. The front is a single closed oval.
 fist=transformed(ellipse(0,15.7,22,15),frame);fist_ink=grow(fist,7)
 arm=shape('M249 317 Q262 311 278 322 L321 348 Q333 357 327 373 Q322 389 307 388 Q291 386 273 373 Q254 361 249 344 Q245 331 249 317Z')
 arm_ink=grow(arm,7)
 # Restore a wide continuous shoulder/arm, modifying only its two original paths.
 body=base[0];i=next(i for i,p in enumerate(body) if p.get('fill','').startswith('var(--fur,') and 234<shape(p.get('d')).bounds[0]<237 and 310<shape(p.get('d')).bounds[1]<320)
 for target,p,fill in ((body[i-1],arm_ink,'#080808'),(body[i],arm,'var(--fur, #2f4ea8)')):
  el=ET.fromstring('<svg>'+svgpath(p,fill)+'</svg>')[0];target.attrib.clear();target.attrib.update(el.attrib)
 (OUT/'mitten-base.svg').write_text(ET.tostring(base,encoding='unicode'))
 # Keep the full original straight handle, but move its rounded end so colour
 # remains visible at least 10 units below the mitten, followed by a dark cap.
 first,last=intervals(fist_ink,(cx-ux*100,cy-uy*100),(ux,uy))[0]
 bottom=last-100
 end_t=bottom+11-13
 start=(cx+ux*(-39),cy+uy*(-39));end=(cx+ux*end_t,cy+uy*end_t)
 # Keep original upper capsule unchanged; extend the lower half on the same axis.
 shaft=shape(f'M{start[0]} {start[1]} L{end[0]} {end[1]}');shaft.stroke(26,pathops.LineCap.ROUND_CAP,pathops.LineJoin.ROUND_JOIN,4);shaft.convertConicsToQuads(.001)
 shaft_ink=grow(shaft,5)
 upper=shape(list(original)[2].get('d'))
 cap_plane=transformed(rect(-30,bottom+8,30,bottom+30),frame)
 cap=op(shaft,cap_plane,pathops.PathOp.INTERSECTION)
 seam=transformed(rect(-16,bottom+7.5,16,bottom+8.7),frame);seam=op(shaft,seam,pathops.PathOp.INTERSECTION)
 # Omit the outline through the wrist so arm and mitten share one colour surface.
 front_ink=op(fist_ink,arm,pathops.PathOp.DIFFERENCE)
 fold=transformed(shape('M-16 13 Q-1 6 15 13 Q-1 9 -16 13Z'),frame)
 blade=next(g for g in racket if g.get('data-pip-part')=='blade')
 xml='<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg"><g data-pip-part="racket">'+svgpath(shaft_ink,'#080808')+svgpath(shaft,'var(--handle, #efa31b)')+svgpath(upper,'#080808')+svgpath(cap,'var(--handle-dark, #985c10)')+svgpath(seam,'#080808')+'<g data-pip-part="flipper">'+svgpath(front_ink,'#080808')+svgpath(fist,'var(--fur, #2f4ea8)')+svgpath(fold,'#080808')+'</g>'+ET.tostring(blade,encoding='unicode')+'</g></svg>'
 (OUT/'mitten-racket.svg').write_text(xml)
 emblem=shape(ET.fromstring(archive('bling-initial'))[0][0].get('d'))
 outline=union([arm_ink,fist_ink,shaft_ink]);bounds=outline.bounds
 origin=(cx-ux*100,cy-uy*100);head_interval=intervals(union(paths(blade)),origin,(ux,uy))[0]
 hand_interval=intervals(fist_ink,origin,(ux,uy))[0];grip_interval=intervals(shaft,origin,(ux,uy))[0]
 neck=hand_interval[0]-head_interval[1];lower=grip_interval[1]-hand_interval[1]
 ref=reference();ra=reference_axis();ru=(-math.sin(math.radians(ra['blade_axis_degrees'])),math.cos(math.radians(ra['blade_axis_degrees'])));rn=(ru[1],-ru[0]);ro=ra['blade_centre']
 paw=grow(ref['paw'],7);pi=intervals(paw,ro,ru)[0];middle=(pi[0]+pi[1])/2;c=(ro[0]+ru[0]*middle,ro[1]+ru[1]*middle)
 cross=intervals(paw,(c[0]-rn[0]*150,c[1]-rn[1]*150),rn,limit=300);pw=cross[-1][1]-cross[0][0]
 upper_grip=op(ref['grip'],rect(0,0,420,313),pathops.PathOp.INTERSECTION);projected=transformed(upper_grip,Transform(rn[0],0,rn[1],1,0,0));gw=projected.bounds[2]-projected.bounds[0]
 visible_intervals=intervals(ref['grip'],ro,ru)
 arm_samples=[]
 dx,dy=44,36;l=math.hypot(dx,dy);an=(-dy/l,dx/l)
 for f in (.25,.4,.55,.7):
  x,y=263+f*dx,330+f*dy;sections=intervals(arm,(x-an[0]*100,y-an[1]*100),an,limit=200)
  arm_samples.append({'fraction':f,'colour_width':sections[-1][1]-sections[0][0]})
 report={'arm_colour_width_samples':arm_samples,'shaft_is_single_path':True,'axis_deviation_above_below':0.,'baseline':'a99eac2','status':'approved and adopted as product','axis_degrees':axis['degrees'],'fist_outline_width':58,'shaft_outline_width':36,'width_ratio':58/36,'fist_colour_width':44,'shaft_colour_width':26,'colour_width_ratio':44/26,'free_neck':neck,'lower_visible_colour_length':lower,'emblem_clearance':clearance(emblem,union([arm_ink,fist_ink,shaft_ink])),'canvas_margin':min(bounds[0],bounds[1],420-bounds[2],420-bounds[3]),'sthings':{'frontal_fist_width':pw,'upper_grip_envelope_width':gw,'ratio':pw/gw,'upper_visible_colour_length':visible_intervals[0][1]-visible_intervals[0][0],'lower_visible_colour_length':visible_intervals[-1][1]-visible_intervals[-1][0],'paw_axis_interval':pi},'product_assets_unchanged':digest()==before}
 assert neck>=20 and lower>=10
 assert report['emblem_clearance']>=12 and report['canvas_margin']>=8
 for raw in (ET.tostring(base,encoding='unicode'),xml):
  tree=ET.fromstring(raw)
  assert all(el.tag.rsplit('}',1)[-1] in ('svg','g','path') for el in tree.iter())
  assert all(not {'stroke','transform','mask','clip-path','filter'}.intersection(el.attrib) for el in tree.iter())
 (OUT/'measurements.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
 assert before==digest() and head==subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
if __name__=='__main__':generate()
