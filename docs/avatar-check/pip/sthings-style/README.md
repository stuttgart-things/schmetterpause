# hank / sthings reference pass

See `docs/avatar-assets.md`, section “hank aligned with sthings”, for provenance,
geometry, cache investigation and replaced test assumptions.

- `sthings-hank-192-app.png`: all six expressions, sthings/hank pairs, on navy.
- `cheek-served-before-420.png` / `cheek-served-after-420.png`: real browser crops
  of the saved actual HTTP SVGs at 420 px. Capture source region (285,175)–(385,275).
- `served-before.svg` / `served-after.svg` and corresponding JSON: exact responses,
  cache policy and verification that every embedded base path matches the build.
  The public random endpoint intentionally has random palette choices; those colours
  may differ between captures. No geometry or colours are replaced in these images.
- `before-base.svg`: prior a9c99c8 base for geometric change-region tests.
- `measurements.json`: exact curve-space distances, overlap, visible grip area.

Regenerate:

```sh
go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip
go run ./scripts/avatar_review.go -sthings-style-only -out docs/avatar-check/pip/sthings-style
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python scripts/measure_hank_pose.py
SP_PLAYWRIGHT_MODULE=/path/to/playwright SP_PREVIEW_BROWSER_BINARY=/path/to/chrome node scripts/hank_style.browser.cjs
```

Eye construction is the actual scaled sthings artwork. New tests compare the
filled artifacts against that source and use eye/brow/beak clearance plus cheek
overlap instead of the old head-clearance/no-sclera requirements.

Minimum 64 px quantised GIF difference: **157**, **Skeptisch/Wütend**.
The physical LED panel and final aesthetic approval remain open.
