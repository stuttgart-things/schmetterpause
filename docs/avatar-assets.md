# Avatar drawings

The active catalogue is `internal/avatar/characters.go`. The empty character
key means sthings; it is also the database default for existing players.
Each other character has `base.svg` and six `face-*.svg` files under
`web/static/img/<character>/`. Every face uses the existing `sthings-face`
class and a `data-face` attribute, including in live SVG and LED GIF output.

blecki retains the provided Character Creator v4 wolf drawing and its pose.
Hank is restored from its workshop branch as the third selectable character.
The Pogo and Kiki workshop drawings remain preserved on separate branches. Face surfaces are separated and additional
expressions are drawn at
the native eye and mouth positions. The supplied jewellery overlays are not
imported: the original sthings jewellery is offered only for sthings. blecki’s ball is purpose-drawn for its own single slot.

## Reproduce the Creator imports

Use the supplied `tischtennis-character-creator-v4 (3)/assets/characters`
directory as input. The release importer processes blecki and Hank and does not
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
handle selects their original colours. Hank retains its separately coloured headband. blecki has no headband and therefore
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


## Hank review

**Hank deliberately has no blush cheeks in any expression.** The six face SVGs
and base contain no cheek group or former cheek colours. Cheeks were removed
because the placement attempts did not suit the drawing. Other avatars, including
the preserved Pogo/Kiki workshop drawings, retain their own cheeks unchanged.

Eye dimensions, centres, sclera-free construction, shared lighting helper,
brows, all lids (including sharpened Skeptical), six beaks, paddle, base and
palettes are unchanged from 7969bdf. The only removed live vector shapes are
the two cheek paths in each face. No new profile field or choice is introduced.

The anchoring helper/constants, cheek measurements JSON, geometric cheek test,
cheek colour/ownership tests, red-cheek test image and unused cheek fixtures
have been removed. The Hank-only GIF palette preservation introduced for cheek
colours is also removed; blecki's separate accessory handling remains intact.
Only an absence test retains the four former colour values, as a forbidden list.

### Pixel separation and remaining review

The minimum pairwise coarse-luminance difference at 64 px is **99 pixels**,
for **Neutral/Angry**, below the requested target of **114**. As explicitly
requested, no expression was adjusted to compensate. This is a disclosed target
shortfall, not a claim that 114 was achieved. The measurement test logs the
shortfall and guards the reported cheek-free baseline against falling below 99;
the existing tests still check six distinct expressions and small-display groups.
All other Go/browser checks remain applicable. Physical LED and aesthetic review
remain manual, especially Neutral versus Angry.

### Reproduce acceptance images

```sh
go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip
```

`docs/avatar-check/pip/index.html` contains all six expressions × three coats at
192/64/28 px on navy and black, eye detail without cheeks, material comparisons
and GIFs. `eyes-before-after-64-black.png` compares the actual saved 7969bdf
64-pixel GIFs in `before-removal-7969bdf/` with the cheek-free runtime output.
These intentional rendered reference fixtures are the only retained old cheek
appearances; no obsolete cheek SVGs are shipped or needed for regeneration.

Black 192-pixel reviews remain palette-quantised, avoiding the former grey
antialiasing intermediates. Navy app previews retain normal antialiasing.
Original/Grey/Blue-grey, coat fallbacks and the Grey/Silver exclusion are unchanged.

The opt-in browser test uses disposable players and runs the existing blecki
regression too:

```sh
SP_RUN_PIP_BROWSER=1 SP_PLAYWRIGHT_MODULE=/path/to/playwright \
SP_PREVIEW_BROWSER_BINARY=/path/to/chrome \
go test ./internal/server -run TestHankBrowser -count=1 -v
```

Set `SP_PREVIEW_SCREENSHOT_DIR` for desktop/mobile captures.

### Hank racket canvas correction

Step 1 (translation) is sufficient. Rotation remains **18 degrees clockwise**
and head scale remains **90%**; neither step 2 nor step 3 was needed.
`PIP_RACKET_HEAD_OFFSET = (-7,34)` is the total offset after the existing baked
rotation/scale, i.e. **(-7,0) relative to the previous accepted head position**.
`PIP_RACKET_GRIP_OFFSET = (-7,-10)` translates the handle and stripes. The gripping
flipper stays in its original position, exposing the chosen handle material
beside/below it. No face or other avatar geometry changes.

The six actual racket paths (shaft, shaft outline, stripes, head and head outline)
are grouped under `data-pip-part="racket"`. The flipper is a separate
`data-pip-part="flipper"` group; it is anatomy rather than racket geometry.
Every racket path is inside [8,412] in both axes, including the handle:

- Head black-outline right bound: 411.0473 → **8.9527 units** right margin.
- Lowest handle-outline bound: 411.5002 → **8.4998 units** bottom margin.
- Minimum distance to head/face contour: **12.772 units**.
- At the original racket-fix review, distance to the 48-unit round proposal was
  **13.501 units**. The later approved 57-unit emblem is checked separately below.
- Exposed selected grip colour after subtracting blade, flipper and black stripes:
  Initially **685.52 square units**, exceeding the 540-unit 30×18 reference area. Its visible
  bounding extent is approximately 48.74×57.85; this is an exposed-area check,
  not a claim of a solid rectangular patch.

Measurements for each path are in `docs/avatar-check/pip/racket-measurements.json`.
The generator asserts margins, clearances and visible handle area. The exact
curve-space test independently reads the output SVG:

```sh
PYTHONDONTWRITEBYTECODE=1 /path/to/vector-tools/bin/python \\
  -m unittest discover -s scripts -p test_hank_racket_geometry.py -v
```

A Go test overscans each path outside the original canvas to catch clipping and
margin violations in normal CI. The gallery exports all expressions at 192 and
64 px on black, plus `racket-colours-192-black.png` and
`racket-colours-64-black.png` for Hellblau, Stahl and Pink. Face separation remains
99 pixels (Neutral/Angry), the documented cheek-free baseline.


### Hank initial emblem (approved smaller/lower placement)

Hank has exactly two options in its existing stored `bling` slot: **Kein Schmuck**
(default) and **Initiale** (`initial`). Snowflake/fish and all other unknown keys
are rejected on submission and fall back to none when reading old profiles.
Saving, random selection and reset reuse the existing infrastructure. `pip` is
still the storage/directory key; no migration or name field was added.

The final black-outlined shield is a **57-unit circle**, centred at **(209.8,366.1)**.
`BELLY_EMBLEM_BOX = (181.3,337.6)–(238.3,394.6)`. A search of the actual belly,
flipper and racket paths allowed about 57.17 units; rounding to 57 provides
clearance tolerance. Measured margins from the entire emblem including outline:

- White belly boundary: **10.052 units**.
- Flippers including the new thumb: **17.785 units**.
- Racket: **12.023 units**.
- All six beaks: more than the required 10 units (checked geometrically).

The shield uses a 5.5-unit black ring. Its colour follows the effective headband;
Ivory and legal Silver instead use the selected coat (including original navy).
There is no separate emblem palette. Grey/Silver remains disallowed by the
existing palette; the review matrix marks its normal Original-pink fallback
instead of enabling that combination. Ivory/Grey demonstrates the grey fallback.

#### Player name / rendering context

`avatar.RenderContext{DisplayName: ...}` is transient and never stored as an avatar
choice. `InitialForName` trims, uppercases the first rune and accepts only A–Z,
Ä, Ö, Ü. Empty/unsupported first characters use a table-tennis-racket symbol.
The client cannot supply the trusted name through a form or query parameter.

| Path | Name source / outcome |
| --- | --- |
| Profile editor and selected thumbnail | Authenticated player from repository → own initial |
| Profile/header figure | Figure's player ID → repository display name |
| Game/live player SVG | Existing `/avatars/{id}.svg` player lookup → initial |
| Player LED/GIF | Same player lookup; context forwarded through every GIF frame |
| Random/demo SVG/GIF | No player identity → racket fallback |
| Unselected character thumbnails | Default figure, no player identity; no initial accessory by default |
| Bare render-library call without context | Racket fallback |
| Check-image script | No ambient player name; matrix supplies Anna, glyph sheet supplies each letter; explicit nameless cell uses racket |

No names are inserted into SVG text. A validated initial selects compiled glyph
paths, and the SVG has no text, runtime font, transform, stroke, masks or images.

#### Glyph source and license

Glyphs are **Barlow Bold**, copyright 2017 The Barlow Project Authors, under the
**SIL Open Font License 1.1**. License: `docs/licenses/Barlow-OFL.txt`. Upstream:
https://github.com/google/fonts/tree/main/ofl/barlow .

`scripts/hank_glyphs.json` stores all 29 decomposed font outlines and the source
font SHA-256. `generate_hank_glyphs.py` creates it once from the bold TTF;
`generate_hank_emblem.py` bakes the final, centred shapes into
`internal/avatar/hank_glyphs.go` and `web/static/img/pip/bling-initial.svg`.
Runtime needs neither the TTF nor Python. Narrow letters retain cap height;
wide glyphs scale down to fit the round coloured interior with their black border.

```sh
PYTHONDONTWRITEBYTECODE=1 /path/to/vector-tools/bin/python scripts/generate_hank_glyphs.py /path/to/Barlow-Bold.ttf
PYTHONDONTWRITEBYTECODE=1 /path/to/vector-tools/bin/python scripts/generate_hank_emblem.py
gofmt -w internal/avatar/hank_glyphs.go
```

#### Grip / thumb correction

The hand detail showed the handle resting beside the flipper. Only a small thumb
was added over the shaft, using a 0.75-scaled local fingertip shape; arm, existing
flipper paths, racket coordinates and the overall outer silhouette are unchanged.
A curve-space XOR test compares the full before/after silhouette and obtains zero
new area. The thumb intersects both the shaft and old hand, visibly wrapping it.

The selected grip colour still has **590.726 square units** exposed after the
blade, flipper, new thumb and black stripes, exceeding the existing 30×18 reference
area (540). All racket paths retain their 8-unit canvas margin and all relevant
head/emblem clearances remain valid. `hand-before-base.svg` is the precise old
base used to reproduce the 420-pixel before/after hand crops.

#### Acceptance output and limitations

```sh
go run ./scripts/avatar_review.go -emblems-only -out docs/avatar-check/pip/emblems
PYTHONDONTWRITEBYTECODE=1 /path/to/vector-tools/bin/python -m unittest discover -s scripts -p 'test_hank_*geometry.py' -v
```

Open `docs/avatar-check/pip/emblems/index.html` for none/initial × Neutral/Angry ×
three coats × five headbands, all 29 letters plus fallback, at 192/64/28 pixels
on navy and black, and 420-pixel before/after hand details. Geometric/glyph/hand
measurements are in `emblems/measurements.json`.

At 64 pixels the shield is **8.69 pixels across** and letter bounding heights are
**3.84–4.86 pixels**. The badge is visible; simple initials are small but identifiable,
while wide letters and umlaut dots have limited readability. At 28 pixels it is
primarily a coloured badge; the letter cannot be reliably read. A real LED check
and subjective approval remain outstanding. No claim of full small-size glyph
legibility is made.

With no emblem, a named A/Ü initial or the racket fallback, the minimum pairwise
64-pixel expression separation remains **99** (Neutral/Angry), meeting this task's
current baseline. The geometric, palette, glyph, persistence/context, random/reset,
SVG restriction and browser checks cover the new slot. Other avatars are unchanged;
blecki keeps its deliberately lowercase display name.
