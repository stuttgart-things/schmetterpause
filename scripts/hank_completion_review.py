#!/usr/bin/env python3
"""Regenerate current hank review and f3d0f10 details; optionally serve without cache."""
import argparse
import hashlib
import html
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'docs/avatar-check/pip'

def regenerate():
    for args in (['-character', 'pip', '-out', str(OUT)],
                 ['-straight-only', '-out', str(OUT / 'straight')],
                 ['-completion-only', '-out', str(OUT / 'completion')]):
        subprocess.run(['go', 'run', './scripts/avatar_review.go', *args], cwd=ROOT, check=True)
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    # Include asset contents as well as HEAD: regenerating an uncommitted artwork
    # revision must never reuse an older image URL.
    digest = hashlib.sha256()
    for asset in sorted((ROOT / 'web/static/img/pip').glob('*.svg')):
        digest.update(asset.read_bytes())
    version = revision + '-' + digest.hexdigest()[:12]
    report = '''# hank – Flosse und pinguinhafte Schnäbel

Vorher: `f3d0f10`. Nachher: korrigierte Greifkontur mit hinterer Flossenpartie, eingerollter Spitze und gegenüberliegendem Daumen.

Die hintere Flossenpartie liegt beidseits hinter dem Griff. Eingerollte Spitze und gegenüberliegender Daumen greifen davor um den Schaft; dazwischen bleibt ein Abschnitt des Griffs sichtbar. Oberer und unterer Griffabschnitt liegen auf derselben Achse. Die sechs Schnäbel besitzen einen spitzen Ober- und dunkleren Unterschnabel, Überrascht eine offene Mundhöhle.

Unterscheidbarkeit im quantisierten 64-px-GIF: **186**, Mindestwert **157**. Auch ohne Namen sowie mit Anna und Ümit: 186.

Alle Detailbilder sind jeweils 420 × 420 px, mit identischen Ausschnitten vorher/nachher. Rot hebt den Griff im Handdetail hervor.

Reproduktion: `python scripts/hank_completion_review.py`; HTTP-Prüfansicht: `python scripts/hank_completion_review.py --serve 8091`.

Historische Zwischenstände bleiben in den Unterordnern dokumentiert. Die aktuelle Prüfansicht ist diese Seite. Ein physischer LED-Test wurde nicht durchgeführt.
'''
    (OUT / 'completion/README.md').write_text(report)
    content = '''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – aktuelle Prüfansicht</title><style>body{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}img{max-width:100%;height:auto}a{color:#65bdff}.pair{display:flex;flex-wrap:wrap;gap:16px}figure{margin:0}figure img{width:420px}code{overflow-wrap:anywhere}</style><h1>hank – Flosse und pinguinhafte Schnäbel</h1>'''
    content += f'<p>Stand: <code>{html.escape(version)}</code>. Vorher: f3d0f10. Unterscheidbarkeit: <strong>186 ≥ 157</strong>.</p><p><a href="completion/README.md">Mess- und Reproduktionsbericht</a></p>'
    for part, label in [('hand', 'Handpartie'), ('neutral', 'Neutral'), ('happy', 'Fröhlich'), ('surprised', 'Überrascht'), ('skeptical', 'Skeptisch'), ('angry', 'Wütend'), ('sad', 'Traurig')]:
        content += f'<h2>{label} – Detail 420 px</h2><div class="pair">'
        for stage, name in [('before', 'Vorher · f3d0f10'), ('after', 'Nachher')]:
            content += f'<figure><figcaption>{name}</figcaption><img src="completion/{part}-{stage}-420.png?v={version}" alt="{label} {name}" width="420" height="420"></figure>'
        content += '</div>'
    for label, path in [('Alle Ausdrücke · 192 px', 'expressions-192-app.png'), ('64 px auf Schwarz', 'expressions-row-64-black.png'), ('28 px auf Schwarz', 'expressions-row-28-black.png'), ('Mit Initiale A', 'straight/sthings-hank-emblem-192-app.png'), ('Alle Grundfarben · 64 px', 'expressions-64-black.png')]:
        content += f'<h2>{label}</h2><img src="{path}?v={version}" alt="{label}">'
    content += '</html>'
    (OUT / 'index.html').write_text(content)
    # The formerly current straight URL must also lead to current reports/images.
    (OUT / 'straight/index.html').write_text('<!doctype html><html lang="de"><meta charset="utf-8"><title>Aktuelle hank Prüfansicht</title><p><a href="../index.html">Aktuelle Prüfansicht: Flosse und sechs Schnäbel</a></p><script>location.replace("../index.html")</script></html>')
    (OUT / 'README.md').write_text(report)
    (OUT / 'straight/README.md').write_text(report)

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
