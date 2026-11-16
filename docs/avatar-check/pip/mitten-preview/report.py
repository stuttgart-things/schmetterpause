#!/usr/bin/env python3
"""Summarize an isolated preview after generate.py, render.go and capture.cjs."""
from pathlib import Path
import hashlib,json
OUT=Path(__file__).resolve().parent
m=json.loads((OUT/'measurements.json').read_text());r=json.loads((OUT/'renderer-verification.json').read_text());b=json.loads((OUT/'browser-verification.json').read_text())
assert b['six_states_visible_and_verified'] and r['minimum_separation_64']>=157
h=hashlib.sha256()
for f in sorted(OUT.glob('*.png')):h.update(f.read_bytes())
h.update((OUT/'mitten-racket.svg').read_bytes());h.update((OUT/'mitten-base.svg').read_bytes());version=h.hexdigest()[:16]
text=f'''# hank – Freigegebener Fäustling nach sthings-Vorbild

**Freigegeben und bytegleich in den Produktstand übernommen.**
Aktueller App-/Pixelvergleich: [Produkt-Fäustling](../mitten-product/index.html).

## sthings als Referenz

Quer zur Schlägerachse, durch den Mittelpunkt des ersten geschlossenen
Faustabschnitts, misst die frontale Faust einschließlich Kontur
**{m['sthings']['frontal_fist_width']:.1f}** Einheiten. Die Hüllbreite der sichtbaren
oberen Stielflächen beträgt **{m['sthings']['upper_grip_envelope_width']:.1f}**:
Verhältnis **{m['sthings']['ratio']:.2f}**. Die Handflächenbox insgesamt ist
(294,313)–(376,418), einschließlich Handgelenk; sie ist nicht mit der frontalen
Faustbreite entlang der gedrehten Messachse gleichzusetzen.

Am Stielzentrum sind oben **{m['sthings']['upper_visible_colour_length']:.1f}**,
unten **{m['sthings']['lower_visible_colour_length']:.1f}** Einheiten Stielfarbe sichtbar.
Die Vorgaben für hank verlangen bewusst mehr: mindestens 20 und ungefähr 10.

Das native sthings-SVG enthält nur einzelne sichtbare Stielfarbflächen; ein
vollständiger durchgehender Stiel hinter der Faust ist dort nicht gezeichnet.
Die später gezeichnete Tintenkontur und die runde Faustfläche fassen den Stiel
optisch von vorn ein. Der hank-Entwurf verwendet dafür einen tatsächlich
durchgehenden Stielpfad hinter der Faust.

## Entwurf

Geschlossene runde Vorderform in Flossenfarbe, Kontur 7 Einheiten und eine einzige
geschwungene Falz. Keine Finger und kein abstehender Daumen. Der breite Arm
verbindet Schulter und Faust; seine gemessenen Farbquerschnitte liegen zwischen
{min(x['colour_width'] for x in m['arm_colour_width_samples']):.1f} und
{max(x['colour_width'] for x in m['arm_colour_width_samples']):.1f} Einheiten.
Die Faustkontur ist am Handgelenk mit dem Arm verbunden, ohne einen schwarzen
Ring oder eine abgesetzte dünne Handwurzel. Der Stiel wird zuerst gezeichnet,
danach die Faust, zuletzt die unveränderte Schlägerfläche.

| Messung | Entwurf |
|---|---:|
| Faust-/Stielbreite einschließlich Kontur | {m['fist_outline_width']:.1f} / {m['shaft_outline_width']:.1f} = **{m['width_ratio']:.2f}** |
| Verhältnis der Farbflächen | {m['fist_colour_width']:.1f} / {m['shaft_colour_width']:.1f} = {m['colour_width_ratio']:.2f} |
| Freier Hals oberhalb der Faust | **{m['free_neck']:.1f}** |
| Stielstück unterhalb der Faust bis zur Kappe | **{m['lower_visible_colour_length']:.1f}** |
| Stielachse | **{m['axis_degrees']:.2f}°** |
| Konstruktionsabweichung oberer/unterer Abschnitt | 0° – derselbe durchgehende Stielpfad |
| Emblemabstand | {m['emblem_clearance']:.3f} ≥ 12 |
| Canvas-Abstand | {m['canvas_margin']:.3f} ≥ 8 |
| Gemessene Unterscheidbarkeit bei 64 px | {r['minimum_separation_64']} ≥ 157 |

Der untere Stielabschluss ist rund und hat eine farblich abgesetzte Kappe.
Schlägerfläche, Gesichter, Emblem, linke Flosse und Rahmung bleiben a99eac2.
Nur die beiden rechten Armpfade der isolierten Basis und der isolierte
Stiel-/Faustaufbau wurden für die Ansicht geändert.

## Prüfbilder und Rendering

- `hand-comparison-420.png`: **sthings | hank a99eac2 | hank Fäustling**, je 420 px.
  Identischer Ausschnitt, roter Stiel zur Sichtkontrolle.
- `expressions-192-navy.png`: alle sechs Ausdrücke, je 192 px.
- `expression-{{neutral,happy,surprised,skeptical,angry,sad}}-192.png`: Einzelbilder.
- `live-expressions.html`: dieselben sechs echten Produktions-`RenderLive`-SVGs
  mit der unveränderten App-CSS-Datei. Chromium hat je genau einen aktiven
  Gesichtszustand geprüft und daraus die 192-px-PNGs aufgenommen.
- `mitten-base.svg`, `mitten-racket.svg`: ausschließlich isolierte Entwurfsassets.
- `measurements.json`, `renderer-verification.json`, `browser-verification.json`:
  Messungen und Renderer-Nachweise.

Die Ausdrucksbilder verwenden **den echten App-Renderer samt App-CSS**, mit
Entwurfsassets ausschließlich in einer privaten In-Memory-Collection. Die
laufende App auf Port 8080 wird für diese Freigabeansicht nicht verändert.
Die Produktübernahme ist freigegeben; aktuelle Tests und Nachweise stehen im Produktbericht.

Reproduktion aus dem Repository mit Vector-Tools-Python:
`python docs/avatar-check/pip/mitten-preview/generate.py`, danach
`go run ./docs/avatar-check/pip/mitten-preview/render.go`,
`node docs/avatar-check/pip/mitten-preview/capture.cjs` mit verfügbarem Playwright,
und `python docs/avatar-check/pip/mitten-preview/report.py`.
Bild-Cachekennung: `{version}`.
'''
(OUT/'README.md').write_text(text)
page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – Fäustling · freigegebener Produktstand</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}a{{color:#65bdff}}</style><h1>hank – Fäustling nach sthings · freigegeben</h1><p>Freigegeben · bytegleich übernommen · siehe aktuellen Produktbericht.</p><p>Faust/Stiel {m['width_ratio']:.2f} · Hals {m['free_neck']:.1f} · unteres Stielstück {m['lower_visible_colour_length']:.1f} · Achse {m['axis_degrees']:.2f}°.</p><p><a href="README.md">Referenzmessung und Entwurfsbericht</a> · <a href="measurements.json">Messwerte</a> · <a href="live-expressions.html">Live-SVGs mit originalem App-CSS</a></p><h2>sthings | hank a99eac2 | hank Fäustling · je 420 px</h2><img src="hand-comparison-420.png?v={version}" alt="Drei Handdetails"><h2>Sechs Ausdrücke aus App-Renderer und App-CSS · je 192 px</h2><img src="expressions-192-navy.png?v={version}" alt="Sechs echte SVG-Ausdrücke im Browser gerendert"><p><a href="../mitten-product/index.html">Produktstand und App-/Pixelvergleich</a></p></html>'''
(OUT/'index.html').write_text(page)
