# Avatar drawings

The active catalogue is `internal/avatar/characters.go`. The empty character
key means sthings; it is also the database default for existing players.
Each other character has `base.svg` and six `face-*.svg` files under
`web/static/img/<character>/`. Every face uses the existing `sthings-face`
class and a `data-face` attribute, including in live SVG and LED GIF output.

blecki retains the provided Character Creator v4 wolf drawing and its pose.
Pip is restored from its workshop branch as the third selectable character.
The Pogo and Kiki workshop drawings remain preserved on separate branches. Face surfaces are separated and additional
expressions are drawn at
the native eye and mouth positions. The supplied jewellery overlays are not
imported: the original sthings jewellery is offered only for sthings. blecki’s ball is purpose-drawn for its own single slot.

## Reproduce the Creator imports

Use the supplied `tischtennis-character-creator-v4 (3)/assets/characters`
directory as input. The release importer processes blecki and Pip and does not
modify its source directory. The workshop branches retain the other imports.

```sh
python3 -m venv /tmp/schmetterpause-vector-tools
/tmp/schmetterpause-vector-tools/bin/pip install fonttools==4.68.0 skia-pathops==0.9.2
PYTHONDONTWRITEBYTECODE=1 /tmp/schmetterpause-vector-tools/bin/python \
  scripts/import_creator_avatars.py '/path/to/creator/assets/characters'
go test ./internal/avatar ./internal/server ./internal/templates
```

These are development tools only. The application embeds the generated SVGs
and still uses its existing Go rasteriser; the runtime requires neither
Python nor a browser. The importer flattens transformations into path
coordinates, preserves aspect ratios on the 420 canvas, resolves material
classes, and writes filled paths without IDs, references, styles or masks.
Boolean operations preserve the native curves when extracting face surfaces.

For the new characters, an empty fur key keeps the original materials.
Choosing a palette colour replaces the main body surface; muzzle, ear and
other material colours remain fixed. The existing `fur` key for headband and
handle selects their original colours. Pip retains its separately coloured headband. blecki has no headband and therefore
has no visible headband control. All player-supplied keys are validated
before saving. Unknown stored characters, including the removed Fynn and
Chomp, fall back to sthings. Their SVG assets are no longer shipped.

ADR-0031 describes the persistence and editor behaviour. Tests check the SVG
contract, six distinct expressions in monochrome, GIF cheering, palette
validation, saving, reset and backwards compatibility.

The profile editor compares the selected figure's six expressions in a labelled
gallery. It follows unsaved character and colour changes. `RenderFace` emits
one complete expression per card, so the gallery does not duplicate the live
six-face SVG in every preview. Expression previews are never persisted.

## blecki review

blecki has a continuous clean forehead and muzzle beneath its expression groups;
the original connected lid/nose ink is separated during import. Native Neutral
eye frames anchor all expressions, and the original side glance is retained.
Neutral and Happy retain their accepted paths. The other expressions use
visible white sclerae, round pupils and white highlights. Half-closed lids
use the coat colour with a pointed black lid edge. Brows taper to both ends
and reach about 12–13 canvas units at their thickest point. Angry lids descend
towards the nose, sad lids outwards; skeptical combines an open eye with a
horizontal half lid. Angry has a closed
mouth without teeth; skeptical has an oblique tapered mouth; sad retains its
rounded frown and downward gaze. Surprised has a thinner eye contour and a
small rose-coloured mouth. Only blecki uses this drawing revision.

blecki’s paddle is a separate assembly, rotated 18 degrees clockwise around the
grip with coordinates baked into the paths. Its head is reduced by 10 percent
and lowered another 10 canvas units. The restored cheek and paddle have at
least 12 units of clearance, checked on the 420 px raster. Hand and handle
stripes follow the rotation. Other characters have independent geometry.
Character selection thumbnails use selectable Light Blue instead of the
reserved red drawing fallback.

blecki's body palette is Original, Sand, White and Chocolate. The three
alternatives remain provisional until their contrast is reviewed. Orange,
Ginger, Cinnamon, Gold and Cream are rejected for blecki in the editor, validation and
random generation. Stored removed coat choices render in original grey.
Other characters keep their existing palettes. Handle and blade palettes are
unchanged pending further review; no automatic white-coat secondary palette
has been introduced.

Export native-size acceptance PNGs, provisional coat comparisons and all
handle/blade choices with:

```sh
go run ./scripts/avatar_review.go -character vex -out /tmp/vex-review
```

The tool writes 28 px, 64 px and larger expression sheets on the app page
background and on black, plus resolved single-expression SVGs. The 64 px black
expression sheet uses the actual GIF palette. It also writes the production
cheer GIF, a six-face review loop, individual 64 px GIFs and
steel/grey plus ivory/paw material-combination sheets. The Zählwerk endpoints
remain `/avatars/{id}.gif` for the cheer loop and `?face=angry` (or another
expression) for a still; `?size=256` requests a larger view.

Physical LED-panel brightness,
colour reproduction and viewing distance still require a hardware check.

## blecki release refinement

blecki is the display name; `vex` remains the stable storage key and asset
folder, so previously saved wolves keep working without a data rewrite.
Only sthings and blecki are selectable in this release. Full workshop snapshots
are preserved remotely on `feat/avatar-workshop`, `feat/pogo-fine-tuning`,
`feat/pip-fine-tuning`, and `feat/kiki-fine-tuning`. Restore those drawings and
adapt their catalogue/tests when continuing their individual refinement.

blecki has four coats (Original, Sand, White, Chocolate) and a single optional
jewellery slot, using the existing `bling` column: none or table-tennis
ball, in that order. The old towel, captain, bandana, moon and tooth keys fall back
to none without changing the stored row. Accessories render between base and
expression and are never borrowed from sthings. The exposed round-ended grip is tested for at least 30 by 18
units of visible chosen colour, including a striped area. The paddle head is
unchanged. Only Surprised's brow paths change; the other face files are exact.

The selected character thumbnail shows the current unsaved or saved materials
and jewellery and updates out of band. Unselected thumbnails keep defaults.

Run `go run ./scripts/avatar_review.go -out docs/avatar-check/vex` to regenerate
the committed acceptance sheets. These cover all six expressions with all four
coats at 192, 64 and 28 px on navy and black; every jewellery option with neutral
and angry at those sizes; every grip colour at profile size; and all fixed
jewellery/coats against all blade colours. Black 64 px sheets use GIF palette
quantisation. GIF endpoint semantics remain unchanged.

## Ball-only accessory

The ball's path coordinates and contour-derived anchor are unchanged. Its
surface and shadow use `--ball` and `--ball-shadow`. `BallColors` defines White
(empty key, #f8f8f8), Orange (#ff8a24), Yellow (#ffe23b). Every shadow scales
each RGB channel by 7/8. `Avatar.BallColor` is persisted in the additive
`player_avatars.ball_color` column, default empty. Unknown/missing keys are
normalised to white on reads/submissions, without rewriting stale reads.
A dormant valid colour survives switching away from the ball, but affects
no other drawing. Reset deletes the choice; rolls colour only selected balls.

The Ballfarbe control updates out of band with the accessory and is visible
only for blecki's ball. Hidden input retains its dormant value. Public SVG,
GIF, expression cards and the selected thumbnail all share the same colours.

The towel and its geometry/generator/studies have been removed entirely.
blecki offers none or the ball; old towel keys use the same no-jewellery
fallback as captain, bandana, moon and tooth. No base change is required.

Regenerate the focused review with:

```sh
go run ./scripts/avatar_review.go -accessories-only \
  -out docs/avatar-check/blecki/accessories
```

Matrices cover all accessories/coats/expressions and every ball colour on all
seven blades and four coats at 192/64/28 px on navy and black. Black 64 px
previews use the production GIF palette. The CSV records fill-to-fill luminance ratios,
without pretending the ratios alone measure outlined-icon readability.
Orange/light-blue (1.10), Orange/steel (1.37), Orange/petrol (1.45) are critical
small-size/LED checks. Yellow/light-blue is 2.00. No palette is automatically
changed; the black ring remains. Physical panel acceptance is still open.

Browser verification is read-only and writes screenshots outside the repo:

```sh
SP_PREVIEW_BASE_URL=http://your-preview-host:8080 \
SP_PREVIEW_PLAYER_ID=your-test-player-uuid \
SP_PREVIEW_COOKIE_FILE=/path/to/netscape-cookies.txt \
SP_PLAYWRIGHT_MODULE=/path/to/node_modules/playwright \
SP_PREVIEW_BROWSER_BINARY=/path/to/chromium \
node scripts/blecki_accessories.browser.cjs
```

It checks the three options, all six expression previews, the selected
thumbnail, every blade, retired-key rejection and mobile layout, and verifies
that preview changes never rewrite the saved SVG. Go handler tests exercise
saving/reset and stale stored choices.


## Pip review

Pip keeps the existing 52 by 62 dark, sclera-free eyes, lighting helper, beaks,
paddle, base and palettes. Neutral remains symmetric and horizontal with
7.56% white-lid coverage; Happy, Surprised, Angry and Sad remain unchanged
except for their cheeks. The original 64-pixel regression threshold remains
114, taken from e2dd9f9. The final measured minimum is **115**.

### Eye-anchored cheeks

`pip_cheek` derives both complete ellipse positions from each expression's
actual visible eye contour, not fixed canvas coordinates:

- `PIP_CHEEK_SIZE = (40,22)`; identical nominal dimensions for left/right.
- `PIP_CHEEK_OFFSET = .25`: centre 25% of eye width towards the outside.
- `PIP_CHEEK_GAP = 10`: cheek top 10 units below visible eye bottom.
- If needed, only an inward quarter-unit search is permitted. Size and y are
  fixed; no clipping or downward escape is used. Current placements need no
  meaningful inward correction.
- Curved-path checks enforce full containment in the visible white face and
  eye clearance between 8 and 14 units. Both cheeks have the same vertical gap.

| Expression | Colour | Width × height | Left/right contour gap |
| --- | --- | --- | --- |
| Neutral | #f5b5ce | 40 × 22 | 11.076 / 11.076 |
| Happy | #f294b8 | 47.2 × 25.96 (+18%) | 11.493 / 11.493 |
| Surprised | #f5b5ce | 40 × 22 | 11.261 / 11.261 |
| Skeptical | #f5b5ce | 40 × 22 | 11.076 / 11.078 |
| Angry | #ec6f74 | 46 × 23.1 (transverse) | 10.974 / 10.974 |
| Sad | #f6d4e0 | 30 × 16.5 (75%) | 11.226 / 11.226 |

All vertical gaps are 10 units, within SVG rounding tolerance of .002 units.
Exact measured centres, sizes and gaps are in `cheek-measurements.json`.
Cheeks remain borderless, expression-owned groups; no profile option, database
field or randomisation choice is introduced. The base contains no cheeks.

### Authorised Skeptical refinement

Only the right horizontal lid and left brow change. The right white lid is
one unit below the eye centre, covering about **52.05%** of the oval. The
left tapered brow spans (104,185) to (188,185), with a stronger -29 quadratic
bend and 16-unit maximum thickness. It is raised and more arched; the right
brow and beak stay unchanged. The lighting helper is byte-for-byte unchanged:
covered large glints shift only vertically until fully visible.

The minimum pairwise coarse-luminance difference in 64-pixel production GIFs
is **115**, above the **114** e2dd9f9 requirement. The reviewed intermediate
open-Neutral state measured 99. Pixel differences do not establish subjective
visual quality or physical LED readability.

### Rendering and acceptance images

Pip's cheek-aware GIF quantisation preserves every unchanged raster pixel,
preventing new cheek colours from recolouring antialiased coat edges. Black
192-pixel review sheets use the same palette quantisation as the 64-pixel LED
preview. The former grey antialiasing intermediates are absent; PNGs are opaque,
and there is no intentional grey outline. Navy app previews retain antialiasing.
No other avatar's assets or output policy are changed.

```sh
go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip
```

`docs/avatar-check/pip/index.html` links:

- Six expressions × three coats, at 192/64/28 px on navy and black.
- `eyes-before-after-64-black.png`: previous open-Neutral working state versus
  new state, expression by expression. Fixtures are in `before-eye-anchored-cheeks/`.
- `eyes-detail-192.png`: eye/brow/cheek detail, with beak/body omitted only in
  the review render.
- `angry-red-options-64-black.png`: coral cheeks with Ruby headband and Red
  grip on Original, Grey and Blue-grey.
- Material sheets, native-size expression rows and runtime-pipeline GIFs.

Run the exact curve-space tests with the vector-tool development venv:

```sh
PYTHONDONTWRITEBYTECODE=1 /path/to/vector-tools/bin/python \
  -m unittest discover -s scripts -p test_pip_geometry.py -v
```

The test reads the generated SVGs and verifies complete ellipses, white-face
containment, 8–14 unit contour gaps, 8–12 unit vertical gaps, common left/right
size and vertical gap, and 20–30% outward horizontal placement. Go tests cover
SVG restrictions, glints, palette fallbacks, red visibility, unchanged GIF pixels,
expression separation and small-display groups.

The real-browser check exercises normal handlers with disposable players,
then runs the existing blecki regression:

```sh
SP_RUN_PIP_BROWSER=1 SP_PLAYWRIGHT_MODULE=/path/to/playwright \
SP_PREVIEW_BROWSER_BINARY=/path/to/chrome \
go test ./internal/server -run TestPipBrowser -count=1 -v
```

Set `SP_PREVIEW_SCREENSHOT_DIR` for desktop/mobile captures. Other review
fixtures remain historical inputs. The cheeks now sit directly beneath the
eyes and remain geometrically separate; final aesthetic acceptance and the
real LED-panel check remain manual. No decision to remove cheeks has been made.
