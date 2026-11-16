#!/usr/bin/env python3
"""Regenerate approved mitten product review; optionally serve it without cache."""
import argparse,subprocess,sys
from http.server import SimpleHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'docs/avatar-check/pip'

def regenerate():
    from hank_mitten_review import generate
    for args in (['-character','pip','-out',str(OUT)],['-straight-only','-out',str(OUT/'straight')],['-completion-only','-out',str(OUT/'flippers')],['-contact-only','-out',str(OUT/'mitten-product')]):
        subprocess.run(['go','run','./scripts/avatar_review.go',*args],cwd=ROOT,check=True)
    for part in ('base','racket'):
        assert (ROOT/f'web/static/img/pip/{part}.svg').read_bytes()==(OUT/f'mitten-preview/mitten-{part}.svg').read_bytes()
    subprocess.run(['go','run','./docs/avatar-check/pip/mitten-preview/render.go'],cwd=ROOT,check=True)
    subprocess.run(['node','docs/avatar-check/pip/mitten-preview/capture.cjs'],cwd=ROOT,check=True)
    m,report,version=generate()
    for target in ('README.md','flippers/README.md','straight/README.md'):(OUT/target).write_text(report)
    import json
    (OUT/'flippers/measurements.json').write_text(json.dumps(m,indent=2)+'\n')
    page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – Produkt-Fäustling</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}a{{color:#65bdff}}</style><h1>hank – Fäustling greift den Stiel</h1><p>Hals {m['free_neck_length']:.1f} · Stielende {m['lower_visible_colour_length']:.1f} · Faust/Stiel {m['width_ratio']:.2f} · Kollinearität {m['collinear_deviation_degrees']:.2f}° · Unterscheidbarkeit {m['minimum_separation']} ≥ 157.</p><p><a href="mitten-product/index.html">Produktbericht und App-/Pixelvergleich</a></p>'''
    for title,name in [('sthings | hank a99eac2 | Fäustling · je 420 px','mitten-product/hand-comparison-420.png'),('Sechs Ausdrücke · je 192 px','mitten-product/expressions-192-navy.png'),('Alle Ausdrücke × drei Grundfarben','flippers/expressions-192-app.png'),('Dieselben 18 Varianten mit Initiale A','flippers/expressions-initial-192-app.png'),('64 px auf Schwarz','expressions-row-64-black.png')]:page+=f'<h2>{title}</h2><img src="{name}?v={version}" alt="{title}">'
    if (OUT/'mitten-product/app-vs-sheet-420.png').exists():page+=f'<h2>Echte App | Referenz · 0 unterschiedliche Pixel</h2><img src="mitten-product/app-vs-sheet-420.png?v={version}" alt="Pixelvergleich">'
    (OUT/'index.html').write_text(page+'</html>')
    (OUT/'straight/index.html').write_text('<!doctype html><meta charset="utf-8"><a href="../index.html">Produkt-Fäustling</a><script>location.replace("../index.html")</script>')

class NoCache(SimpleHTTPRequestHandler):
    def __init__(self,*args,**kwargs):super().__init__(*args,directory=str(OUT),**kwargs)
    def end_headers(self):self.send_header('Cache-Control','no-store, max-age=0');super().end_headers()

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--serve',type=int);p.add_argument('--serve-only',action='store_true');a=p.parse_args()
    if not a.serve_only:regenerate()
    if a.serve:ThreadingHTTPServer(('0.0.0.0',a.serve),NoCache).serve_forever()
