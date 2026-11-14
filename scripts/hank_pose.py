"""Rigid racket targets, in final canvas space before the shared frame is baked."""
import math
from racket_axis import reference_axis
BLADE_SCALE=.91
BLADE_CENTRE=(346.1,269.8)
BLADE_RIM=6.5
REF_AXIS=reference_axis()
AXIS_DEGREES=REF_AXIS['blade_axis_degrees']
BLADE_MASS_CENTRE=(BLADE_CENTRE[0]+BLADE_SCALE*(REF_AXIS['blade_centre'][0]-348),BLADE_CENTRE[1]+BLADE_SCALE*(REF_AXIS['blade_centre'][1]-228.5))
def axis_at_y(y):
 return (BLADE_MASS_CENTRE[0]-math.tan(math.radians(AXIS_DEGREES))*(y-BLADE_MASS_CENTRE[1]),y)
GRIP_START=axis_at_y(316.)
GRIP_END=axis_at_y(392.)
GRIP_WIDTH=26.
GRIP_RIM=5.
HAND_BODY='M284 370 Q296 369 304 361 Q312 352 325 355 Q344 358 344 374 Q343 387 331 391 Q316 397 306 386 L291 389 Q281 387 277 378Z'
HAND_THUMB='M310 367 Q321 361 334 368 Q323 365 310 367Z'
