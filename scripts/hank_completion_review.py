#!/usr/bin/env python3
"""Regenerate the full-fist review and optionally serve the review tree."""
import argparse,subprocess,sys
from http.server import SimpleHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'docs/avatar-check/pip'

def regenerate():
    subprocess.run([sys.executable,'scripts/hank_full_fist_preview.py'],cwd=ROOT,check=True)
    subprocess.run(['go','run','./docs/avatar-check/pip/full-fist-preview/render.go'],cwd=ROOT,check=True)
    version=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
    page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – volle Faust</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}</style><h1>hank – volle Faust</h1><p><a href="full-fist-preview/README.md">Messwerte und Umsetzung</a></p><h2>sthings | aktueller Stand | neue Faust · 420 px</h2><img src="full-fist-preview/hand-comparison-420.png?v={version}"><h2>Sechs Ausdrücke · 192 px</h2><img src="full-fist-preview/expressions-192.png?v={version}"></html>'''
    (OUT/'index.html').write_text(page)

class NoCache(SimpleHTTPRequestHandler):
    def __init__(self,*args,**kwargs):super().__init__(*args,directory=str(OUT),**kwargs)
    def end_headers(self):self.send_header('Cache-Control','no-store, max-age=0');super().end_headers()

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--serve',type=int);p.add_argument('--serve-only',action='store_true');a=p.parse_args()
    if not a.serve_only:regenerate()
    if a.serve:ThreadingHTTPServer(('0.0.0.0',a.serve),NoCache).serve_forever()
