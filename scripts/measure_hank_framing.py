"""Export anatomical head masks for reproducible measurements on 420px renders."""
import json,sys
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape,svgpath,rect,op
from test_hank_racket_geometry import union,paths
from hank_frame import SCALE,OFFSET_X,OFFSET_Y,CREST_HEIGHT_FACTOR,CREST_TOP_BEFORE,CREST_TOP_AFTER
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'docs/avatar-check/pip/framing';OUT.mkdir(exist_ok=True)
meta={}
for label,key,before in [('sthings','sthings',False),('hank-before','pip',True),('hank-after','pip',False),('blecki','vex',False)]:
 root=ET.parse(OUT/'before-base.svg' if before else ROOT/'web/static/img'/key/'base.svg').getroot()
 if key=='sthings':
  # Pharaoh headdress belongs to the head; the racket right of the cheek does not.
  p=op(union(paths(root)),shape('M0 0H420V159H333V300H0Z'),pathops.PathOp.INTERSECTION)
  eye=181.5;chin=297;bottom=418
 elif key=='pip':
  p=union(paths(root[0]));cut=280 if before else 280*SCALE+OFFSET_Y
  p=op(p,rect(0,0,420,cut),pathops.PathOp.INTERSECTION)
  eye=225.4 if before else 225.4*SCALE+OFFSET_Y
  chin=315 if before else 315*SCALE+OFFSET_Y;bottom=420
 else:
  # The first two black paths are the cleaned native head/body and rebuilt cheek.
  p=op(union(paths(root)[:2]),rect(0,0,420,260),pathops.PathOp.INTERSECTION)
  eye=201.5;chin=310;bottom=420
 (OUT/(label+'-head-mask.svg')).write_text('<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg">'+svgpath(p,'#ffffff')+'</svg>')
 meta[label]={'eyes_y':round(eye,3),'eyes_percent':round(eye/4.2,2),'chin_y':round(chin,3),'bottom_crop_y':bottom,'chin_to_bottom':round(bottom-chin,3)}
meta['frame']={'scale':SCALE,'offset':[OFFSET_X,OFFSET_Y],'crest_shortening_percent':(1-CREST_HEIGHT_FACTOR)*100,'crest_top_before_native':CREST_TOP_BEFORE,'crest_top_after_native':CREST_TOP_AFTER,'native_canvas_bottom_after':(420-OFFSET_Y)/SCALE}
(OUT/'framing-metadata.json').write_text(json.dumps(meta,indent=2,ensure_ascii=False)+'\n')
