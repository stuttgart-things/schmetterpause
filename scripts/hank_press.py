"""Selected B: reflected native flipper presses a straight grip against the side.
All geometry is final canvas space; exported SVG contains filled paths only.
"""
import math
import pathops
from fontTools.misc.transform import Transform
from fontTools.svgLib.path import parse_path
from hank_frame import transformed
from racket_axis import capsule_axis

SCALE=.8
ANGLE=-180
POSITION=(251.,332.)
RIM=7.

def polygon(points):
    p=pathops.Path()
    parse_path('M'+' L'.join(f'{x} {y}' for x,y in points)+'Z',p.getPen())
    return p

def outline(p,width=RIM):
    border=pathops.Path(p)
    border.stroke(2*width,pathops.LineCap.ROUND_CAP,pathops.LineJoin.ROUND_JOIN,4)
    border.convertConicsToQuads(.001)
    return pathops.op(p,border,pathops.PathOp.UNION)

def flipper(left):
    p=transformed(left,Transform().rotate(math.radians(ANGLE)).scale(-SCALE,SCALE))
    x,y,_,_=p.bounds
    p=transformed(p,Transform(1,0,0,1,POSITION[0]-x,POSITION[1]-y))
    return p,outline(p)

def bury_grip(shaft,hand):
    """Return a clipping plane and hidden binding inside the complete flipper."""
    a=capsule_axis(shaft);ux,uy=a['unit'];cx,cy=a['centre']
    # The end must fit the outer 36-unit grip, not merely cross its centre.
    buried=None
    for j in range(-50,101):
        t=j*.5;x,y=cx+ux*t,cy+uy*t
        if all(hand.contains((x+uy*k,y-ux*k)) for k in range(-19,20)):
            buried=t+3;break
    assert buried is not None,'cannot bury full grip width in flipper'
    for t in (buried-2,buried-1,buried):
        assert all(hand.contains((cx+ux*t+uy*k,cy+uy*t-ux*k)) for k in range(-18,19))
    plane=polygon([(cx+ux*t+uy*n,cy+uy*t-ux*n) for t,n in [(-200,-180),(-200,180),(buried,180),(buried,-180)]])
    binding=polygon([(cx+ux*t+uy*n,cy+uy*t-ux*n) for t,n in [(buried-1.5,-11),(buried-1.5,11),(buried-.5,11),(buried-.5,-11)]])
    return plane,binding
