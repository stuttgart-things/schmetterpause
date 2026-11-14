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
# Final canvas coordinates: broad shoulder, rounded transverse tip, one fold.
FLIPPER_BODY='M247 321 Q268 304 291 326 L330 359 Q348 367 344 381 Q342 386 329 386 Q315 387 299 401 Q284 408 272 400 Q258 389 258 369 Q242 347 247 321Z'
FLIPPER_FOLD='M263 330 Q268 328 272 333 L307 361 Q311 365 308 368 Q305 370 301 367 L266 337 Q262 334 263 330Z'
FLIPPER_RIM=7.
