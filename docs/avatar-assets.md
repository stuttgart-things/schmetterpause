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

Pip is a penguin restored from `feat/pip-fine-tuning`; no other workshop
characters are enabled. Its base, native Neutral expression and Happy expression
retain the Creator design. The other eyes use visible sclerae, glossy pupils and
filled upper lids. Skeptical has an open left eye, a raised brow and a horizontal
right lid; Angry lids slope inwards; Sad lids slope outwards with a downward gaze.
The beak remains the native filled shape across expressions.

The initial body palette is Original (navy), Sand and Chocolate. Pale coats are
excluded to preserve the white face/bib separation. Palette removal falls back
to Original for stored Pip profiles. Headband, handle and blade palettes remain
shared. Pip has no jewellery control or jewellery assets. Existing persistence,
reset, random selection, live expression previews and avatar SVG/GIF endpoints
are reused without a database migration.

Reproduce the PNG sheets and GIFs through the runtime renderer:

```sh
go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip
```

Open `docs/avatar-check/pip/index.html` to compare expressions at 192, 64 and
28 pixels on navy and black, material choices, the 64-pixel animation and a
larger viewing copy. Physical LED-panel contrast and the aesthetic acceptance
of Sand/Chocolate remain manual checks. The dark native coat has reduced
silhouette contrast against black; eyes, bib and headband remain visible.
