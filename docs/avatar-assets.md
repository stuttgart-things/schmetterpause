# Avatar drawings

The active catalogue is `internal/avatar/characters.go`. The empty character
key means sthings; it is also the database default for existing players.
Each other character has `base.svg` and six `face-*.svg` files under
`web/static/img/<character>/`. Every face uses the existing `sthings-face`
class and a `data-face` attribute, including in live SVG and LED GIF output.

Blecki retains the provided Character Creator v4 wolf drawing and its pose.
The Pogo, Pip and Kiki workshop drawings are preserved on separate branches. Face surfaces are separated and additional
expressions are drawn at
the native eye and mouth positions. The supplied jewellery overlays are not
imported: the original sthings jewellery is offered only for sthings. Blecki’s four
new pieces are purpose-drawn for its own single slot.

## Reproduce the Creator imports

Use the supplied `tischtennis-character-creator-v4 (3)/assets/characters`
directory as input. The release importer processes Blecki only and does not
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
handle selects their original colours. Blecki has no headband and therefore
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

## Blecki review

Blecki has a continuous clean forehead and muzzle beneath its expression groups;
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
small rose-coloured mouth. Only Blecki uses this drawing revision.

Blecki’s paddle is a separate assembly, rotated 18 degrees clockwise around the
grip with coordinates baked into the paths. Its head is reduced by 10 percent
and lowered another 10 canvas units. The restored cheek and paddle have at
least 12 units of clearance, checked on the 420 px raster. Hand and handle
stripes follow the rotation. Other characters have independent geometry.
Character selection thumbnails use selectable Light Blue instead of the
reserved red drawing fallback.

Blecki's body palette is Original, Sand, White and Chocolate. The four
alternatives remain provisional until their contrast is reviewed. Orange,
Ginger, Cinnamon, Gold and Cream are rejected for Blecki in the editor, validation and
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

## Blecki release refinement

Blecki is the display name; `vex` remains the stable storage key and asset
folder, so previously saved wolves keep working without a data rewrite.
Only sthings and Blecki are selectable in this release. Full workshop snapshots
are preserved remotely on `feat/avatar-workshop`, `feat/pogo-fine-tuning`,
`feat/pip-fine-tuning`, and `feat/kiki-fine-tuning`. Restore those drawings and
adapt their catalogue/tests when continuing their individual refinement.

Blecki has four coats (Original, Sand, White, Chocolate) and a single optional
jewellery slot, using the existing `bling` column. Bandana, captain's armband,
moon pendant and rounded wolf tooth have fixed colours and independent filled
contours. They render between base and expression. Jewellery is never borrowed
from sthings. The exposed round-ended grip is tested for at least 30 by 18
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
