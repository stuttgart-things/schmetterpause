# blecki acceptance review

Regenerate from the repository root:

sh
go run ./scripts/avatar_review.go -character vex -out docs/avatar-check/vex


The storage key and folder remain vex; the public name is blecki.

- Expressions: all six faces, all four coats, at native 192, 64 and 28 px.
- Jewellery: both slot options (none and ball), neutral and angry, all four coats,
  at native 192, 64 and 28 px.
- Grips: all seven handle choices, all four coats, at 192 px.
- Jewellery/blades: both accessories against every coat and blade colour.
- Every sheet is exported on navy (#0a0a12) and pure black.
- Black 64 px sheets use the actual GIF palette quantisation.
- Individual SVGs/GIFs, the production cheer loop and a six-face review loop
  are included. The review loop does not change game-controlled expressions.

## Observations and remaining review

The enlarged grip distinguishes red, dark wood and ivory at profile size.
All fixed jewellery colours retain a filled black outline across the four
coats. No palette colours were silently changed. The blade head coordinates
remain identical to the accepted workshop revision.

Ball colours are white (default), orange and yellow with calculated shadows.
The ball's geometry and contour-derived anchor are unchanged. The focused
review includes all colour/blade combinations and a fill-contrast CSV.
Orange on light-blue, steel and petrol needs particular review at 28 px/LED;
the unchanged black ring helps, but does not establish physical LED contrast.

Only the table-tennis ball remains as blecki's optional accessory. Towel,
captain band and neck pieces are retired. Old keys fall back to no jewellery.
The active ball geometry and all base/face/material paths remain unchanged.

Physical LED contrast is still unverified: these are software previews,
not photographs of a real panel. In particular, dark wood and moss on black
should be checked on the device. No runtime hides jewellery automatically.
