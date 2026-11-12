# Hank initial emblem acceptance review

Run from the repository root:

```sh
go run ./scripts/avatar_review.go -emblems-only -out docs/avatar-check/pip/emblems
```

The shield's full black-outlined diameter is **57 units**, centred at (209.8,366.1).
It fits entirely within the unchanged white belly. This is the enlarged version
of the approved smaller/lower placement, not the original 90–110 unit request.

- `emblems-{192,64,28}-{app,black}.png`: none/initial, Neutral/Angry, three coats,
  Original/Gold/Ruby/Ivory/Silver headbands; name Anna supplied explicitly.
- `letters-{192,64,28}-{app,black}.png`: A–Z, Ä/Ö/Ü and the racket fallback.
  Each cell is the whole figure at the stated size, not an enlarged emblem.
- `hand-before-420.png` / `hand-after-420.png`: a 420-pixel hand detail before
  and after the added thumb. `hand-before-base.svg` preserves the exact old base.
- `measurements.json`: actual margins, selected grip colour area and glyph sizes.

The matrix's `Silber→Orig.*` marks the existing forbidden Grey/Silver combination:
normal read-time fallback changes the headband to Original pink. It is not silently
made selectable. Ivory on Grey demonstrates the body-colour emblem fallback;
Silver demonstrates it on the two coats where Silver is permitted.

Name availability: player-backed editor, header, live/player SVG and player GIF
use trusted repository display names through transient RenderContext. Random/demo
figures and renderer calls without context use the racket symbol. Unselected
catalogue figures have no owner name. The matrix deliberately supplies Anna;
the letter sheet supplies each glyph as a sample name.

Font: Barlow Bold, SIL Open Font License 1.1, docs/licenses/Barlow-OFL.txt.
The font is never loaded at runtime; generated filled path data covers all 29
letters. Source outline JSON is scripts/hank_glyphs.json.

At 64 px the shield is about 8.7 pixels wide, with roughly 4–5-pixel letters.
The badge is clear but wide letters and umlaut dots are not reliably readable
at that size. At 28 px it is principally a coloured badge. Physical LED readability
and aesthetic approval remain manual checks.

Fallback correction: a single solid white paddle silhouette, head upper right and
short handle lower left (about 45 degrees), with a separate black filled outline.
`fallback-oq-{192,64}-black.png` compares O, Q and the nameless fallback.
The geometry test rejects inner subpaths and checks the shared interior margin.
