# hank – contour cleanup

Regenerate the preserved before/after detail and all six expressions:

```sh
go run ./scripts/avatar_review.go -cheek-contour-only -out docs/avatar-check/pip/cheek-contour
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python -m unittest discover -s scripts -p 'test_hank_*geometry.py' -v
```

`before-base.svg` is the exact previous base. The spur belonged to the first
black body path, inherited from the originally fused head/racket silhouette.
The old cut at y=220 retained the former racket's upper shoulder. The cut now
starts at y=212 and the cheek joins at (340.3075,212) with a smooth quadratic curve.
Only the first two body outline paths change; other body surfaces and every
racket, grip, flipper, face and emblem path stay unchanged.

- `cheek-before-420.png`, `cheek-after-420.png`: the same 100×100-unit region
  (285,175)–(385,275), enlarged to 420 px, black background.
- `expressions-192-black.png`: all six expressions after cleanup.
- Geometry checks enforce the local edit region, no protrusion, >=12 units of
  clearance for each racket/flipper path, and no unassigned tiny fragments.
  Preserved native body shading is explicitly recognised as intentional.

Template names are centrally lowercase; player names and uppercase initials
are unaffected. Blue grip on Blue-grey fur remains a documented critical
combination, with no automatic recolouring.
