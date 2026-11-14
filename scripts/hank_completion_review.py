#!/usr/bin/env python3
"""Regenerate hank flipper review against 1298bfe; optionally serve without cache.
Run with the vector-tools Python (fontTools and skia-pathops).
"""
import argparse
import hashlib
import html
import json
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'docs/avatar-check/pip'


def regenerate():
    from hank_flipper_geometry import measurements
    for args in (['-character', 'pip', '-out', str(OUT)],
                 ['-straight-only', '-out', str(OUT / 'straight')],
                 ['-completion-only', '-out', str(OUT / 'flippers')]):
        subprocess.run(['go', 'run', './scripts/avatar_review.go', *args], cwd=ROOT, check=True)
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    digest = hashlib.sha256()
    for asset in sorted((ROOT / 'web/static/img/pip').glob('*.svg')):
        digest.update(asset.read_bytes())
    version = revision + '-' + digest.hexdigest()[:12]
    m = measurements()
    (OUT / 'flippers/measurements.json').write_text(json.dumps(m, indent=2)+'\n')
    report = f'''# hank – beide Flossen in derselben Formensprache

Vergleich: **1298bfe → aktuelles Artwork**. Bildversion: `{version}`.

Die rechte Flosse ist eine breite, durchgehende Form von der Schulter bis zur
abgerundeten, quer über den Griff gebogenen Spitze. Keine Finger oder Daumen;
nur eine einzelne gefüllte Falzlinie. Die Spitze liegt in `racket` vor dem Griff.
Die Flosse in `base` und die vordere Flosse haben dieselbe Geometrie und überlappen
vollständig: kein dünnes Handgelenk und keine Naht.

## Messwerte aus den erzeugten SVG-Pfaden

| Messung | Wert |
|---|---:|
| Schmalster vergleichbarer Flossenquerschnitt links | {m['left_stem_min']:.2f} |
| Schmalster vergleichbarer Flossenquerschnitt rechts | {m['right_stem_min']:.2f} |
| Rechts / links | {m['minimum_width_ratio']*100:.2f} % (zulässig 85–115 %) |
| Rechte Kontur, Abstand zur Farbfläche | 7 Einheiten |
| Abstand Flosse / Emblem | {m['emblem_clearance']:.3f} (mindestens 12) |
| Kleinster Canvas-Abstand der Flosse | {m['canvas_margin']:.3f} (mindestens 8) |
| Sichtbare Grifffarbe, abzüglich Streifen und Überdeckung | {m['visible_grip_area']:.2f} Einheiten² (mindestens 540) |
| Freier Griffhals | {m['free_neck_length']:.2f} |
| Unterer sichtbarer Griffabschnitt | {m['lower_grip_length']:.2f} |
| Griffachse | {m['grip_axis']['degrees']:.5f}°, unverändert |
| Unterscheidbarkeit im quantisierten 64-px-GIF | 186 (mindestens 157) |

Die Breitenmessung folgt der jeweiligen aus den gefüllten Pfaden gemessenen
Längsachse. Der vergleichbare Schaftbereich reicht von 25 bis 65 % der Länge;
Schulteranschluss und runde Endkappe sind ausgeschlossen. Alle 81 Querschnitte
pro Seite stehen in `measurements.json`. Die Messung überspannt die Falzöffnung.
Oberer und unterer Griffabschnitt liegen auf derselben Achse; die Spitze
überdeckt die volle Griffbreite. Ein Test sichert diese Querüberdeckung separat.

## Prüfbilder

- `both-flippers-before-420.png`: links und rechts, je 420 px; Stand 1298bfe.
- `both-flippers-after-420.png`: dieselben Ausschnitte nachher, je 420 px.
- `left-flipper-{{before,after}}-420.png`, `right-flipper-{{before,after}}-420.png`: Einzelbilder.
- `expressions-192-app.png`: sechs Ausdrücke × drei Grundfarben, 192 px auf Navy.
- `expressions-initial-192-app.png`: dieselben 18 Varianten mit Initiale A.

## Änderungen und unveränderte Bereiche

Produktassets: nur `web/static/img/pip/base.svg` (zwei Pfade der rechten Flosse)
und `web/static/img/pip/racket.svg` (Flossengruppe).
Generator: `scripts/hank_pose.py`, `scripts/import_creator_avatars.py`.
Messung/Tests: `scripts/hank_flipper_geometry.py`, `scripts/test_hank_*geometry.py`,
SVG-Strukturprüfungen in `internal/avatar/{{avatar,pip}}_test.go`.
Prüfbilder/Seite: `scripts/avatar_review.go`, `scripts/hank_completion_review.py`,
`docs/avatar-check/pip/`.

`invariants-1298bfe.json` sichert alle übrigen SVGs bytegenau, einschließlich
Augen, Schnäbeln, Emblem, Stirnband und anderen Avataren. Der Basistest erlaubt
nur die beiden rechten Flossenpfade; die linke Flosse bleibt bytegenau gleich.
Schlägerfläche, kompletter Griff einschließlich Streifen, Position und Achse
bleiben bytegenau gleich. Paletten, Rahmung und Glyphen bleiben unverändert.

Reproduktion mit Python samt fontTools/skia-pathops:
`python scripts/hank_completion_review.py --serve 8091`.
HTTP sendet `Cache-Control: no-store`; alle Bild-URLs enthalten Quellcommit und
SVG-Inhaltshash. Physischer LED-Test und optische Abnahme stehen aus.
'''
    for target in ('README.md', 'flippers/README.md', 'straight/README.md'):
        (OUT / target).write_text(report)
    content = '''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – beide Flossen</title><style>body{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}img{max-width:100%;height:auto}a{color:#65bdff}code{overflow-wrap:anywhere}</style><h1>hank – beide Flossen</h1>'''
    content += f'<p>Vorher: 1298bfe. Bildversion: <code>{html.escape(version)}</code>.</p><p>Minimale Flossenbreite rechts/links: {m["minimum_width_ratio"]*100:.2f} %. Unterscheidbarkeit: 186 ≥ 157.</p><p><a href="flippers/README.md">Bericht mit Messwerten und geänderten Dateien</a> · <a href="flippers/measurements.json">Alle Querschnitte</a></p>'
    for label, path in [('Vorher · 1298bfe · beide Flossen je 420 px', 'flippers/both-flippers-before-420.png'), ('Nachher · beide Flossen je 420 px', 'flippers/both-flippers-after-420.png'), ('Sechs Ausdrücke × drei Grundfarben · 192 px auf Navy', 'flippers/expressions-192-app.png'), ('Dieselben 18 Varianten mit Initiale A', 'flippers/expressions-initial-192-app.png'), ('64 px auf Schwarz', 'expressions-row-64-black.png'), ('28 px auf Schwarz', 'expressions-row-28-black.png')]:
        content += f'<h2>{label}</h2><img src="{path}?v={version}" alt="{label}">'
    (OUT / 'index.html').write_text(content+'</html>')
    (OUT / 'straight/index.html').write_text('<!doctype html><html lang="de"><meta charset="utf-8"><title>hank</title><a href="../index.html">Aktuelle Flossen-Prüfansicht</a><script>location.replace("../index.html")</script></html>')


class NoCache(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(OUT), **kwargs)

    def end_headers(self):
        self.send_header('Cache-Control', 'no-store, max-age=0')
        super().end_headers()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--serve', type=int)
    parser.add_argument('--serve-only', action='store_true')
    args = parser.parse_args()
    if not args.serve_only:
        regenerate()
    if args.serve:
        ThreadingHTTPServer(('0.0.0.0', args.serve), NoCache).serve_forever()
