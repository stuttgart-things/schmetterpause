# blecki: ball-only accessory

The active choices are no jewellery and table-tennis ball. The ball has White
(default), Orange and Yellow surfaces with a calculated same-hue shadow.
The colour control appears only for the ball. Colour is persisted independently,
retained when no accessory is chosen, rolled for selected balls and reset to
white. Missing/unknown colour keys fall back to white. Towel, captain, bandana,
moon and wolf tooth are retired and render as no jewellery without rewriting
old stored rows. Their assets/generator geometry are removed.

## Geometry and rendering

`RACKET_BALL_ANCHOR = (0.65, 0.39)` is relative to the unchanged coloured blade
bounds, resolving to approximately (365.835, 320.261). Radius 18 gives a
36-unit outer diameter, with at least eight units of edge clearance. The
accepted ball paths are unchanged; only surface/shadow fills are variable.
Shadows scale each RGB channel by 7/8. Base, all six faces, racket, grip,
material palettes and other avatars are unchanged.

## Reproduce

```sh
go run ./scripts/avatar_review.go -accessories-only \
  -out docs/avatar-check/blecki/accessories
```

- `accessories-*`: none/ball, neutral/angry and all four coats.
- `ball-blades-*`: three ball colours, seven blades, four coats and both faces.
- Native 192, 64 and 28 px, on navy and pure black.
- Black 64 px sheets use the production GIF renderer.
- `ball-colour-contrast.csv`: diagnostic fill-to-fill luminance ratios.

## Open visual acceptance

Orange/light-blue (1.10), Orange/steel (1.37), Orange/petrol (1.45) have low
fill-only luminance contrast. Yellow/light-blue is 2.00. The unchanged black
ring and hue differences help, but 28 px and real LED contrast need review.
No colours are silently altered. White remains the default. These are software
previews, not photographs of a physical LED panel.

## Implementation and checks

Avatar/model/renderer, additive ball_colour persistence migration, handlers,
conditional templ control, generator, review exporter, Go/PostgreSQL/browser
tests and ADR-0031 describe the ball-only release. The source column is
`ball_color` and the form field is `ballColor`; empty means White.
