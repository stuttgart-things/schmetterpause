#!/usr/bin/env python3
"""Prepare Creator v4 drawings for Schmetterpause's SVG/GIF pipeline.

Usage: python scripts/import_creator_avatars.py /path/to/creator/assets/characters
Development dependencies: fonttools and skia-pathops. No runtime dependency.
Only the four accepted Creator characters are imported.
"""

import argparse
import math
from fontTools.misc.transform import Transform
from fontTools.pens.transformPen import TransformPen
from pathlib import Path
from fontTools.pens.svgPathPen import SVGPathPen
import pathops
import creator_svg

INK = "#080808"
WHITE = "#f8f8f8"
FACES = ["neutral", "happy", "surprised", "skeptical", "angry", "sad"]
# Native face coordinates after flattening each drawing onto the 420 canvas.
SPECS = {
    "pogo": dict(
        eyes=[(147, 251, 19, 23), (278, 230, 17, 21)],
        mouth=(212, 296),
        regions=[(110, 217, 177, 280), (246, 204, 309, 257), (171, 248, 261, 327)],
        holes={("body", 1): {3}},
        remove={"eye-highlight", "tongue"},
        native="happy",
        background="#1e1d2e",
    ),
    "pip": dict(
        eyes=[(146, 219, 21, 21), (260, 190, 19, 23)],
        mouth=(213, 240),
        regions=[(110, 186, 181, 247), (227, 156, 287, 225), (177, 205, 250, 267)],
        holes={("muzzle", 0): {1, 2}},
        remove={"eye-pupil", "eye-highlight", "beak", "beak-shadow"},
        native="neutral",
        background="#fff6f3",
    ),
    "vex": None,  # Native eye-frame repair and expression builder below.
    "kiki": dict(
        eyes=[(144, 274, 20, 27), (241, 254, 19, 26)],
        mouth=(196, 313),
        regions=[(113, 227, 175, 307), (213, 219, 268, 287), (162, 272, 234, 338)],
        holes={("body", 0): {4, 5, 6, 7, 8}},
        remove={"eye-white", "eye-pupil", "eye-highlight", "nose", "tongue"},
        native="happy",
        background="#fcf8f5",
    ),
}


def shape(d):
    from fontTools.svgLib.path import parse_path

    p = pathops.Path()
    parse_path(d, p.getPen())
    return p


def rect(x1, y1, x2, y2):
    return shape(f"M{x1} {y1} H{x2} V{y2} H{x1}Z")


def ellipse(cx, cy, rx, ry):
    return shape(
        f"M{cx-rx} {cy} C{cx-rx} {cy-ry*1.3333} {cx+rx} {cy-ry*1.3333} {cx+rx} {cy} C{cx+rx} {cy+ry*1.3333} {cx-rx} {cy+ry*1.3333} {cx-rx} {cy}Z"
    )


def op(a, b, kind):
    return pathops.op(a, b, kind)


def svgpath(p, fill):
    if not p.verbs:
        return ""
    p.convertConicsToQuads(0.01)
    pen = SVGPathPen(None, ntos=lambda n: f"{n:.3f}".rstrip("0").rstrip("."))
    p.draw(pen)
    return f'  <path fill="{fill}" fill-rule="evenodd" d="{pen.getCommands()}"/>\n'


def paint(d, fill=INK):
    return svgpath(shape(d), fill)


def oval(cx, cy, rx, ry, fill=INK):
    return svgpath(ellipse(cx, cy, rx, ry), fill)


def colour(role, fill):
    if role in ["ink", "eye-pupil"] or fill.lower() in [
        "#050606",
        "#050608",
        "#070609",
        "#080809",
        "#050706",
        "#070a02",
    ]:
        return INK
    if role in ["eye-white", "eye-highlight"]:
        return WHITE
    if role == "body":
        return f"var(--fur, {fill})"
    if role == "headband":
        return f"var(--headdress, {fill})"
    if role == "headband-shadow":
        return f"var(--headdress-dark, {fill})"
    if role in ["paddle-handle", "paddle-handle-end", "paddle-grip-highlight"]:
        return f"var(--handle, {fill})"
    if role in ["paddle-face", "paddle-rim"] and fill.lower() not in ["#070609"]:
        return "var(--paddle, #c82828)"
    return fill


def eye(cx, cy, rx, ry, expression, bg):
    if expression == "happy":
        return paint(
            f"M{cx-rx} {cy+4} Q{cx} {cy-ry} {cx+rx} {cy+4} L{cx+rx-3} {cy+10} Q{cx} {cy-ry+12} {cx-rx+3} {cy+10}Z",
            WHITE if bg == "#1e1d2e" else INK,
        )
    if expression == "sad":
        return paint(
            f"M{cx-rx} {cy-3} Q{cx} {cy+ry*.75} {cx+rx} {cy-3} L{cx+rx-2} {cy+6} Q{cx} {cy+ry+3} {cx-rx+2} {cy+6}Z",
            WHITE if bg == "#1e1d2e" else INK,
        )
    s = oval(cx, cy, rx, ry, INK) + oval(cx, cy, rx - 5, ry - 5, WHITE)
    s += oval(cx, cy + 2, rx * 0.49, ry * 0.67, INK) + oval(
        cx - rx * 0.19, cy - ry * 0.25, 3.3, 4.1, WHITE
    )
    # Lids and brows are filled silhouettes. Keep both neutral eyes open.
    if expression in ["angry", "skeptical", "neutral"]:
        amount = (
            8
            if expression == "neutral"
            else (ry + 3 if expression == "skeptical" else 15)
        )
        s += paint(
            f"M{cx-rx-1} {cy-ry-2} H{cx+rx+1} V{cy-ry+amount} Q{cx} {cy-ry+amount-4} {cx-rx-1} {cy-ry+amount}Z",
            bg,
        )
    return s


def brows(spec, expression):
    s = ""
    for i, (cx, cy, rx, ry) in enumerate(spec["eyes"]):
        y = cy - ry - 5
        if expression == "angry":
            dy = 12 if i == 0 else -12
            s += paint(
                f"M{cx-rx} {y-6} L{cx+rx} {y+dy} L{cx+rx} {y+dy+7} L{cx-rx} {y+1}Z"
            )
        elif expression == "sad":
            dy = -11 if i == 0 else 11
            s += paint(
                f"M{cx-rx} {y+2} L{cx+rx} {y+dy} L{cx+rx} {y+dy+6} L{cx-rx} {y+8}Z"
            )
        elif expression == "skeptical":
            lift = 10 if i == 0 else -2
            s += paint(
                f"M{cx-rx} {y-lift} Q{cx} {y-10-lift} {cx+rx} {y-lift} L{cx+rx} {y+6-lift} Q{cx} {y-4-lift} {cx-rx} {y+6-lift}Z"
            )
    return s


def visible(scene):
    result = []
    covered = pathops.Path()
    for r, f, p in reversed(scene):
        result.append((r, f, op(p, covered, pathops.PathOp.DIFFERENCE)))
        covered = op(covered, p, pathops.PathOp.UNION)
    return list(reversed(result))


def mouth(name, spec, expression, original):
    x, y = spec["mouth"]
    if name == "pip":
        beak = [(r, f, p) for r, f, p in original if r in ["beak", "beak-shadow"]]
        outline = pathops.Path()
        outline.addPath(beak[0][2])
        outline.stroke(6, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4)
        return svgpath(outline, INK) + "".join(
            svgpath(p, colour(r, f)) for r, f, p in beak
        )
    if name == "kiki":
        nose = "".join(svgpath(p, colour(r, f)) for r, f, p in original if r == "nose")
    else:
        nose = paint(
            f"M{x-13} {y-29} Q{x} {y-35} {x+13} {y-29} Q{x+14} {y-22} {x} {y-18} Q{x-14} {y-22} {x-13} {y-29}Z"
        )
    if expression == "surprised":
        return nose + oval(x, y, 9, 12)
    if expression in ["sad", "angry"]:
        return nose + paint(
            f"M{x-18} {y+8} Q{x} {y-14} {x+18} {y+8} L{x+15} {y+12} Q{x} {y-4} {x-15} {y+12}Z"
        )
    if expression == "skeptical":
        return nose + paint(
            f"M{x-17} {y+8} Q{x} {y-2} {x+19} {y+2} L{x+19} {y+8} Q{x} {y+4} {x-17} {y+14}Z"
        )
    if expression == "happy":
        return (
            nose
            + paint(
                f"M{x-22} {y-6} Q{x} {y+3} {x+22} {y-6} Q{x+17} {y+21} {x} {y+22} Q{x-17} {y+21} {x-22} {y-6}Z"
            )
            + oval(x, y + 14, 9, 5, "#fa557d")
        )
    return nose + paint(
        f"M{x-17} {y} Q{x} {y+14} {x+18} {y-3} L{x+19} {y+3} Q{x} {y+21} {x-17} {y+7}Z"
    )


# Measured on the native Neutral eye silhouettes, in the flattened 420 grid.
# (centre x/y, half width/height). The native side glance is intentional.
VEX_EYES = ((182, 201, 34, 33), (285, 202, 29, 30))
VEX_FUR = "var(--fur, #817d93)"


def vex_eye(anchor, state, index):
    cx, cy, rx, ry = anchor
    if state == "happy":
        return paint(
            f"M{cx-rx+2} {cy+5} C{cx-rx*.6} {cy-22} {cx+rx*.6} {cy-22} {cx+rx-2} {cy+5} "
            f"Q{cx+rx+1} {cy+12} {cx+rx-8} {cy+14} "
            f"C{cx+rx*.45} {cy-2} {cx-rx*.45} {cy-2} {cx-rx+8} {cy+14} "
            f"Q{cx-rx-1} {cy+12} {cx-rx+2} {cy+5}Z"
        )
    # A visible white sclera surrounds a round pupil in every open eye.
    # Lids use the coat colour with a separate tapered ink edge, never a
    # solid black half-disc. Neutral and happy are kept on their own paths.
    if state == "surprised":
        rx *= 0.92
        ry *= 0.92
    outer = ellipse(cx, cy, rx, ry)
    inner = ellipse(cx, cy, rx - 5, ry - 5)
    # The lid slopes, not the amount of ink, distinguish the negative moods.
    # Screen y grows down: anger drops towards the nose; sadness drops outwards.
    lid_ends = None
    if state == "angry":
        lid_ends = (cy - ry * 0.45, cy + 1) if index == 0 else (cy + 1, cy - ry * 0.45)
    elif state == "sad":
        lid_ends = (cy + 3, cy - ry * 0.5) if index == 0 else (cy - ry * 0.5, cy + 3)
    elif state == "skeptical":
        lid_ends = (
            (cy - ry * 0.7, cy - ry * 0.7 + 3) if index == 0 else (cy - 6, cy - 6)
        )
    s = ""
    if lid_ends is not None:
        yl, yr = lid_ends
        mid = (yl + yr) / 2
        below = shape(
            f"M{cx-rx-2} {yl} Q{cx} {mid+2} {cx+rx+2} {yr} V{cy+ry+2} H{cx-rx-2}Z"
        )
        s += svgpath(op(outer, below, pathops.PathOp.DIFFERENCE), VEX_FUR)
        outer = op(outer, below, pathops.PathOp.INTERSECTION)
        inner = op(inner, below, pathops.PathOp.INTERSECTION)
    s += svgpath(outer, INK) + svgpath(inner, WHITE)
    if state == "surprised":
        pupil_x, pupil_y, radius = cx - 2, cy + 2, 14
    elif state == "sad":
        pupil_x, pupil_y, radius = (
            cx,
            cy + ry - (17 if index == 0 else 16),
            11 if index == 0 else 10,
        )
    elif state == "angry":
        pupil_x, pupil_y, radius = (
            cx - 3,
            cy + 15 if index == 0 else cy + 14,
            12 if index == 0 else 10,
        )
    elif state == "skeptical" and index == 1:
        pupil_x, pupil_y, radius = cx - 3, cy + 14, 10
    else:
        pupil_x, pupil_y, radius = cx - 4, cy + 5, 15
    pupil = ellipse(pupil_x, pupil_y, radius, radius)
    highlight = ellipse(pupil_x - 3.5, pupil_y - 4, 4.2, 4.5)
    s += svgpath(op(pupil, inner, pathops.PathOp.INTERSECTION), INK)
    s += svgpath(op(highlight, inner, pathops.PathOp.INTERSECTION), WHITE)
    if lid_ends is not None:
        yl, yr = lid_ends
        mid = (yl + yr) / 2
        s += paint(
            f"M{cx-rx+2} {yl} Q{cx} {mid-9} {cx+rx-2} {yr} Q{cx} {mid+17} {cx-rx+2} {yl}Z"
        )
    else:
        # A strong pointed upper rim follows Neutral's graphic eye construction.
        s += vex_brow(cx - rx + 2, cy - ry * 0.45, cx + rx - 2, cy - ry * 0.5, -14, 13)
    return s


def vex_brow(x1, y1, x2, y2, bend=0, width=10.5):
    # Two quadratics meet in pointed ends; at the midpoint the ink is width
    # units thick. There are no rectangular ends or heavy black blocks.
    mx, my = (x1 + x2) / 2, (y1 + y2) / 2 + bend
    return paint(f"M{x1} {y1} Q{mx} {my-width} {x2} {y2} Q{mx} {my+width} {x1} {y1}Z")


def vex_brows(state):
    if state == "angry":
        return vex_brow(149, 166, 214, 190, -1, 13) + vex_brow(
            256, 190, 317, 166, -1, 13
        )
    if state == "sad":
        return vex_brow(149, 181, 213, 157, -2, 12.5) + vex_brow(
            255, 157, 316, 181, -2, 12.5
        )
    if state == "skeptical":
        return vex_brow(148, 157, 214, 155, -17, 13) + vex_brow(
            257, 187, 315, 187, -1, 12.5
        )
    if state == "surprised":
        return vex_brow(149, 155, 214, 153, -12, 13) + vex_brow(
            256, 155, 316, 155, -11, 13
        )
    return ""


def vex_mouth(state):
    if state == "happy":
        # A crooked grin, one canine, and a tongue keep the wolf's bite.
        return (
            paint("M193 251 Q224 272 259 258 Q251 289 225 291 Q201 287 193 251Z")
            + paint("M202 260 L215 265 L211 277Z", WHITE)
            + paint("M215 284 Q226 272 240 282 Q230 291 215 284Z", "#fa557d")
        )
    if state == "surprised":
        return oval(226, 271, 10, 14) + oval(226, 271, 6, 10.5, "#a53e56")
    if state == "skeptical":
        return paint("M208 275 C228 280 244 268 264 267 C245 280 228 286 208 275Z")
    if state == "angry":
        return paint(
            "M194 284 Q197 274 204 273 L253 273 Q262 275 266 284 L258 282 Q256 279 251 280 L205 280 Q201 279 199 284Z"
        )
    if state == "sad":
        return paint(
            "M213 284 C215 271 225 269 230 274 C235 268 244 274 246 283 L241 286 C237 278 234 278 230 282 C225 277 220 278 218 287Z"
        )
    raise ValueError(state)


def convert_vex(source, target):
    original = creator_svg.paths(source, "vex")
    native_visible = visible(original)
    muzzle = next(p for r, _, p in original if r == "muzzle")
    mouth_contour = list(muzzle.contours)[1]
    clean_muzzle = pathops.Path()
    clean_muzzle.addPath(list(muzzle.contours)[0])
    # Repair one continuous forehead plane. The former small rectangles left
    # two lid tips above the eye boxes and a long tip between the eyes.
    repair = shape(
        "M142 159 C192 150 272 151 318 160 L321 254 C270 259 200 257 140 250Z"
    )
    nose = shape(
        "M235 237 C243 232 258 233 270 235 C275 238 278 243 275 247 C271 254 263 259 257 260 C249 259 239 253 236 248 C234 244 233 240 235 237Z"
    )

    def transformed(p, transform):
        result = pathops.Path()
        p.draw(TransformPen(result.getPen(), transform))
        return result

    def outline(p, width):
        border = pathops.Path(p)
        border.stroke(width, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4)
        border.convertConicsToQuads(0.01)
        return op(p, border, pathops.PathOp.UNION)

    rotation = (
        Transform().translate(281, 376).rotate(math.radians(18)).translate(-281, -376)
    )
    shrink = Transform().translate(330, 300).scale(0.9).translate(-330, -300)
    # The native silhouette includes equipment. Cut it out before rebuilding
    # the cheek and the separate, rotated paddle/paw assembly.
    remove = rect(268, 235, 420, 420)
    for index in (6, 9, 23, 24, 25, 26):
        remove = op(remove, outline(original[index][2], 16), pathops.PathOp.UNION)
    cheek_ink = shape(
        "M292 213 C313 210 339 220 352 233 Q357 237 352 242 L337 252 L344 260 Q331 271 316 274 Q301 294 277 302 Q247 314 223 310 L219 299 Q260 300 279 284 Q285 264 292 247Z"
    )
    cheek_light = shape(
        "M300 236 C316 234 332 233 343 237 L326 250 L332 257 Q324 265 310 266 Q294 285 273 294 Q248 305 223 302 L222 294 Q256 294 272 280 Q280 260 286 246Z"
    )
    blank = []
    for index, (r, f, p) in enumerate(original):
        if r in {"eye-pupil", "eye-white", "eye-highlight"} or index in {
            1,
            6,
            9,
            22,
            23,
            24,
            25,
            26,
        }:
            continue
        if index == 0:
            blank.append((r, f, op(p, remove, pathops.PathOp.DIFFERENCE)))
            blank.append(("ink", INK, cheek_ink))
        elif r == "muzzle":
            blank.append(("body", "#817d93", repair))
            blank.append((r, f, clean_muzzle))
            blank.append((r, f, cheek_light))
            blank.append(("ink", INK, nose))
        else:
            blank.append((r, f, p))
    # A round-ended grip leaves a full 18-unit colour band beyond the paw.
    shaft = shape("M315 338 L237 417")
    shaft.stroke(28, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4)
    shaft.convertConicsToQuads(0.01)
    shaft_colour = shape("M315 338 L237 417")
    shaft_colour.stroke(18, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4)
    shaft_colour.convertConicsToQuads(0.01)
    blank.append(("ink", INK, transformed(shaft, rotation)))
    blank.append(("paddle-handle", "#e79a48", transformed(shaft_colour, rotation)))
    for stripe in (
        "M300 346 L313 358 L309 363 L296 351Z",
        "M243 405 L258 419 L253 424 L238 410Z",
    ):
        blank.append(("ink", INK, transformed(shape(stripe), rotation)))
    blade = transformed(
        transformed(transformed(original[22][2], shrink), rotation),
        Transform().translate(0, 10),
    )
    blank.append(("ink", INK, outline(blade, 13)))
    blank.append(("paddle-face", "#c82828", blade))
    paw = op(original[6][2], original[9][2], pathops.PathOp.UNION)
    blank.append(("ink", INK, transformed(outline(paw, 12), rotation)))
    for index in (6, 9):
        r, f, p = original[index]
        blank.append((r, f, transformed(p, rotation)))
    target.mkdir(parents=True, exist_ok=True)

    def save(name, body):
        (target / (name + ".svg")).write_text(
            '<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg">\n'
            + body
            + "</svg>\n"
        )

    save("base", "".join(svgpath(p, colour(r, f)) for r, f, p in blank))
    jewellery = {
        "bandana": (
            paint(
                "M176 314 Q205 324 237 314 L253 322 L245 337 L234 332 L214 366 Q208 372 202 363 L179 337 L166 341 L160 332Z"
            )
            + paint(
                "M178 321 Q205 333 234 321 L240 329 L208 358 L181 331 L167 335 L168 330Z",
                "#e43c43",
            )
            + paint("M238 319 L249 323 L243 331 L237 328Z", "#e43c43")
        ),
        "captain": (
            paint("M126 319 Q140 311 156 320 L157 353 Q142 363 126 353Z")
            + paint("M132 324 Q141 320 150 325 L151 349 Q142 354 132 349Z", "#f8cf55")
            + paint(
                "M146 328 C133 324 131 349 146 346 L146 340 C139 345 138 332 146 334Z"
            )
        ),
        "moon": (
            paint(
                "M179 314 Q209 338 239 314 L242 321 Q226 336 214 341 L214 346 L204 346 L204 341 Q187 333 176 321Z"
            )
            + paint(
                "M221 342 C192 333 188 373 214 376 Q229 375 232 361 C212 371 204 350 221 342Z"
            )
            + paint(
                "M212 348 C196 347 198 370 214 370 Q220 369 223 366 C212 365 207 357 212 348Z",
                "#f8d86e",
            )
        ),
        "wolf-tooth": (
            paint(
                "M179 314 Q209 338 239 314 L242 321 Q225 337 214 341 L214 347 L204 347 L204 341 Q186 332 176 321Z"
            )
            + paint(
                "M199 341 Q210 336 221 343 L221 353 Q218 368 208 376 Q198 370 196 355Z"
            )
            + paint(
                "M204 346 Q211 343 216 347 L216 353 Q214 363 208 368 Q202 363 202 354Z",
                "#f2ead8",
            )
        ),
    }
    for key, body in jewellery.items():
        if key != "captain":
            # Keep the neck pieces on the light chest, above the crossed paws.
            import xml.etree.ElementTree as ET

            neck = Transform().translate(0, 314).scale(1, 0.65).translate(0, -314)
            body = "".join(
                svgpath(transformed(shape(el.get("d")), neck), el.get("fill"))
                for el in ET.fromstring("<svg>" + body + "</svg>")
            )
        save(
            "bling-" + key,
            f'<g class="sthings-bling" data-bling="{key}">\n' + body + "</g>\n",
        )
    neutral_region = op(
        rect(145, 160, 219, 237), rect(252, 162, 319, 233), pathops.PathOp.UNION
    )
    neutral = "".join(
        svgpath(op(p, neutral_region, pathops.PathOp.INTERSECTION), colour(r, f))
        for r, f, p in native_visible
    )
    neutral += svgpath(mouth_contour, INK)
    for face in FACES:
        if face == "neutral":
            body = neutral
        else:
            body = "".join(
                vex_eye(anchor, face, i) for i, anchor in enumerate(VEX_EYES)
            )
            body += vex_brows(face) + vex_mouth(face)
        save(
            "face-" + face,
            f'  <g class="sthings-face" data-face="{face}">\n' + body + "  </g>\n",
        )


def convert(source, name, target):
    if name == "vex":
        return convert_vex(source, target)
    spec = SPECS[name]
    original = creator_svg.paths(source, name)
    # Move native facial holes out of the coloured surfaces. They become
    # independently replaceable expression paths, not holes in the body.
    blank = []
    counts = {}
    for role, fill, p in original:
        index = counts.get(role, 0)
        counts[role] = index + 1
        if role in spec["remove"]:
            continue
        omit = spec["holes"].get((role, index), set())
        if omit:
            clean = pathops.Path()
            clean.fillType = p.fillType
            for j, c in enumerate(p.contours):
                if j not in omit:
                    clean.addPath(c)
            p = clean
        blank.append((role, fill, p))
    if name == "pip":
        blank.append(("muzzle", "#fff6f3", ellipse(213, 236, 34, 30)))
    region = rect(*spec["regions"][0])
    for b in spec["regions"][1:]:
        region = op(region, rect(*b), pathops.PathOp.UNION)
    # Keep continuous background surfaces under each face to avoid seams in the
    # LED rasteriser; the removed native features are not hidden underneath.
    base = "".join(svgpath(p, colour(r, f)) for r, f, p in blank)
    backdrop = "".join(
        svgpath(op(p, region, pathops.PathOp.INTERSECTION), colour(r, f))
        for r, f, p in visible(blank)
    )

    def save(file, body):
        (target / (file + ".svg")).write_text(
            '<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg">\n'
            + body
            + "</svg>\n"
        )

    target.mkdir(parents=True, exist_ok=True)
    save("base", base)
    for face in FACES:
        if face == spec["native"]:
            body = "".join(
                svgpath(op(p, region, pathops.PathOp.INTERSECTION), colour(r, f))
                for r, f, p in visible(original)
            )
        else:
            body = backdrop
            for index, (cx, cy, rx, ry) in enumerate(spec["eyes"]):
                bg = spec["background"]
                if name in ["vex", "kiki"]:
                    bg = f"var(--fur, {bg})"
                eye_expression = (
                    "neutral" if face == "skeptical" and index == 0 else face
                )
                body += eye(cx, cy, rx, ry, eye_expression, bg)
            body += brows(spec, face) + mouth(name, spec, face, original)
        # Clip the re-drawn features to the same interior face areas. Native ears,
        # cheeks, headbands and paddle contours remain untouched.
        import xml.etree.ElementTree as ET

        clipped = ""
        for el in ET.fromstring("<svg>" + body + "</svg>"):
            p = shape(el.get("d"))
            clipped += svgpath(
                op(p, region, pathops.PathOp.INTERSECTION), el.get("fill")
            )
        save(
            "face-" + face,
            f'  <g class="sthings-face" data-face="{face}">\n' + clipped + "  </g>\n",
        )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("--target", type=Path, default=Path("web/static/img"))
    args = parser.parse_args()
    for name in ("vex",):
        convert(args.source, name, args.target / name)
