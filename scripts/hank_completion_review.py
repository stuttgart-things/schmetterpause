#!/usr/bin/env python3
"""Regenerate selected B80 product review; serve with no-store on request."""
import argparse
import importlib.util
from http.server import SimpleHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
import subprocess
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'docs/avatar-check/pip'

def regenerate():
    for args in (['-character','pip','-out',str(OUT)],['-straight-only','-out',str(OUT/'straight')],['-completion-only','-out',str(OUT/'flippers')]):
        subprocess.run(['go','run','./scripts/avatar_review.go',*args],cwd=ROOT,check=True)
    spec=importlib.util.spec_from_file_location('press_review',OUT/'hand-variants/generate.py')
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    m,report,version=module.generate()
    import json
    (OUT/'flippers/measurements.json').write_text(json.dumps(m,indent=2)+'\n')
    for target in ('README.md','flippers/README.md','straight/README.md'):
        (OUT/target).write_text(report)
    page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – Produkt B80</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}a{{color:#65bdff}}</style><h1>hank – B drückt den Schläger an · 80 %</h1><p>Breite {m['flipper_width_ratio']*100:.2f} % · Emblem {m['emblem_clearance']:.2f} · Canvas {m['canvas_margin']:.2f} · Griffhals {m['free_neck_length']:.1f} · Unterscheidbarkeit {m['minimum_separation']} ≥ 157.</p><p><a href="hand-variants/README.md">Bericht und geänderte Dateien</a> · <a href="hand-variants/measurements-B.json">Messwerte</a></p>'''
    for title,path in [('sthings | 1298bfe | Produkt B80 · 192 px','hand-variants/comparison-192-navy.png'),('Schläger-/Flossenpartie · je 420 px','hand-variants/hand-racket-comparison-420.png'),('Alle sechs Ausdrücke × drei Grundfarben · 192 px Navy','flippers/expressions-192-app.png'),('Dieselben 18 Varianten mit Initiale A','flippers/expressions-initial-192-app.png'),('64 px auf Schwarz','expressions-row-64-black.png'),('28 px auf Schwarz','expressions-row-28-black.png')]:
        page+=f'<h2>{title}</h2><img src="{path}?v={version}" alt="{title}">'
    (OUT/'index.html').write_text(page+'</html>')
    (OUT/'straight/index.html').write_text('<!doctype html><html lang="de"><meta charset="utf-8"><a href="../index.html">Produkt B80</a><script>location.replace("../index.html")</script></html>')

class NoCache(SimpleHTTPRequestHandler):
    def __init__(self,*args,**kwargs):super().__init__(*args,directory=str(OUT),**kwargs)
    def end_headers(self):self.send_header('Cache-Control','no-store, max-age=0');super().end_headers()

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--serve',type=int);p.add_argument('--serve-only',action='store_true');a=p.parse_args()
    if not a.serve_only:regenerate()
    if a.serve:ThreadingHTTPServer(('0.0.0.0',a.serve),NoCache).serve_forever()
