"""Single baked framing operation for hank base, faces, equipment and glyphs."""
import pathops
from fontTools.misc.transform import Transform
from fontTools.pens.transformPen import TransformPen
from fontTools.svgLib.path import parse_path
from fontTools.pens.svgPathPen import SVGPathPen
import xml.etree.ElementTree as ET

SCALE=1.2
OFFSET_X=-27.21127
CREST_JOIN_Y=60.0
CREST_HEIGHT_FACTOR=.6  # 40% shorter, keeping every original tuft tip.
CREST_TOP_BEFORE=11.053
CREST_TOP_AFTER=CREST_JOIN_Y+(CREST_TOP_BEFORE-CREST_JOIN_Y)*CREST_HEIGHT_FACTOR
OFFSET_Y=8-SCALE*CREST_TOP_AFTER
EMBLEM=(212.1,347.,20.)  # 48 units final diameter, fully above the tighter crop.
MATRIX=Transform(SCALE,0,0,SCALE,OFFSET_X,OFFSET_Y)

def transformed(p,t):
 out=pathops.Path();p.draw(TransformPen(out.getPen(),t));return out

def rectangle(x1,y1,x2,y2):
 p=pathops.Path();parse_path(f'M{x1} {y1}H{x2}V{y2}H{x1}Z',p.getPen());return p

def framed(p,crest=False,crop=False):
 if crest:
  upper=pathops.op(p,rectangle(-100,-100,600,CREST_JOIN_Y),pathops.PathOp.INTERSECTION)
  lower=pathops.op(p,rectangle(-100,CREST_JOIN_Y,600,600),pathops.PathOp.INTERSECTION)
  upper=transformed(upper,Transform(1,0,0,CREST_HEIGHT_FACTOR,0,CREST_JOIN_Y*(1-CREST_HEIGHT_FACTOR)))
  p=pathops.op(upper,lower,pathops.PathOp.UNION)
 p=transformed(p,MATRIX)
 if crop:p=pathops.op(p,rectangle(0,0,420,420),pathops.PathOp.INTERSECTION)
 return p

def bake(body,crest=False,crop=False):
 root=ET.fromstring('<svg>'+body+'</svg>')
 for el in list(root.iter('path')):
  p=pathops.Path();parse_path(el.get('d'),p.getPen())
  p=framed(p,crest,crop)
  pen=SVGPathPen(None);p.draw(pen);el.set('d',pen.getCommands())
 for parent in root.iter():
  for child in list(parent):
   if child.tag=='path' and not child.get('d'):parent.remove(child)
 return ''.join(ET.tostring(el,encoding='unicode') for el in root)
