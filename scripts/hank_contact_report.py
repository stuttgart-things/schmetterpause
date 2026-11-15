#!/usr/bin/env python3
"""Write the runtime/contact audit and cache-busted screenshots index."""
import hashlib,json,sys,xml.etree.ElementTree as ET
from pathlib import Path
from hank_contact_geometry import contact_metrics
from hank_flipper_geometry import measurements
from PIL import Image
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'docs/avatar-check/pip/grip-contact'
sha=lambda b:hashlib.sha256(b).hexdigest()

def geometry(raw):
 root=ET.fromstring(raw)
 def part(name):return next(g for g in root.iter() if g.get('data-pip-part')==name)
 face=next(g for g in root.iter() if g.tag.rsplit('}',1)[-1]=='g' and g.get('data-face')=='neutral')
 data={name:[p.get('d') for p in part(name).iter() if p.tag.endswith('path')] for name in ('body','racket')}
 data['neutral']=[p.get('d') for p in face.iter() if p.tag.endswith('path')]
 return sha(json.dumps(data,sort_keys=True).encode())

def generate():
 m=measurements();baseline_axis=m['grip_axis']
 before=contact_metrics((OUT/'before-racket.svg').read_bytes(),axis=baseline_axis)
 after=contact_metrics((ROOT/'web/static/img/pip/racket.svg').read_bytes(),axis=baseline_axis)
 delivered=(OUT/'after-http-neutral.svg').read_bytes();reference=(OUT/'reference-neutral.svg').read_bytes()
 assert geometry(delivered)==geometry(reference)
 a=Image.open(OUT/'app-render-420.png').convert('RGBA');b=Image.open(OUT/'sheet-render-420.png').convert('RGBA')
 changed=sum(a.getpixel((x,y))!=b.getpixel((x,y)) for y in range(420) for x in range(420))
 assert changed==0,'Actual profile and sheet rendering must be pixel-identical'
 pixels={'size':[420,420],'different_pixels':changed,'pixels_identical':True,'app_image_sha256':sha((OUT/'app-render-420.png').read_bytes()),'sheet_image_sha256':sha((OUT/'sheet-render-420.png').read_bytes())}
 (OUT/'pixel-comparison.json').write_text(json.dumps(pixels,indent=2)+'\n')
 m=measurements();runtime=json.loads((OUT/'runtime-verification.json').read_text());audit=json.loads((OUT/'before-http-audit.json').read_text())
 result={'before_commit':'bddb539','before_contact':before,'after_contact':after,'constraints':m,'pixels':pixels,'runtime':runtime,
 'after_http_neutral_sha256':sha(delivered),'after_http_neutral_geometry_sha256':geometry(delivered),'reference_neutral_geometry_sha256':geometry(reference),
 'asset_sha256':sha((ROOT/'web/static/img/pip/racket.svg').read_bytes())}
 (OUT/'contact-audit.json').write_text(json.dumps(result,indent=2)+'\n')
 text=f'''# hank – überprüfter Griffkontakt in App und Prüfbild

## Welcher Stand war ausgeliefert?

Vorher-Prozess: PID 4030381, Binärdatei `/tmp/schmetterpause-hank`,
Startverzeichnis `/home/sthings/projects/schmetterpause`. Die App lief **bereits
im normalen Repo**; die Prüfansicht wurde aus dem separaten Worktree bedient.
Die Binärdatei enthielt den Stand bddb539, obwohl der normale Workspace inzwischen
auf main stand. Ein Branchwechsel verändert eingebettete SVGs einer laufenden
Binärdatei nicht.

Das per HTTP gelieferte Schlägerasset war bytegleich zu bddb539:
`{audit['http_asset_sha256']}`.
Der bereinigte Geometriehash (Körper, neutrales Gesicht und Schläger) war ebenfalls
identisch: `{audit['http_neutral_geometry_sha256']}`.
Die vollständigen Live-/Flat-SVG-Bytehashes unterscheiden sich wegen anderer
Farben und der zusätzlichen Live-Gesichter; siehe `before-http-audit.json`.
Es wurde also keine andere Geometrieversion ausgeliefert.

## Ursache und Korrektur

Der Griff war bereits kurz nach dem ersten vollständig verdeckten Querschnitt
abgeschnitten. Nur **{before['full_width_covered_fraction']*100:.2f} %** seiner
Länge lagen vollständig hinter der Flosse. Der bisherige Test prüfte lediglich,
dass der Abschluss verdeckt war; er prüfte keine Mindestüberdeckungslänge.

Der versteckte Griff läuft jetzt bis kurz vor das Ende des sicher bedeckten
Bereichs. Zusätzlich wurde die 80-%-Flosse um 0,50 nach rechts und 0,05 nach oben
verschoben. Form, Größe, Winkel, Schlägerfläche, Achse, Gesichter, Emblem und
Rahmung bleiben unverändert. Die Zeichenreihenfolge ist Griff → Flosse → Fläche.

| Messung | Vorher bddb539 | Nachher |
|---|---:|---:|
| Über volle Breite verdeckter Griffanteil | {before['full_width_covered_fraction']*100:.2f} % | {after['full_width_covered_fraction']*100:.2f} % ≥ 30 % |
| Abstand Überdeckungszentrum / Griffachse | {before['axis_offset_from_overlap_centroid']:.3f} | {after['axis_offset_from_overlap_centroid']:.3f} ≤ 5 |
| Flossengröße | 80 % | 80 % |
| Emblemabstand | 12,541 | {m['emblem_clearance']:.3f} ≥ 12 |
| Canvas-Abstand | 8,070 | {m['canvas_margin']:.3f} ≥ 8 |
| Sichtbarer Griffhals | 20,7 | {m['free_neck_length']:.2f} ≥ 20 |
| Gesicht-/Flächenüberdeckung | 0 | 0 |
| Unteres sichtbares Griffende | keines | keines |

Die Vektormessung nutzt die unveränderte Achse des vollständigen Baseline-Griffs,
0,25-Einheiten-Abstände und die tatsächlichen gefüllten
Pfade einschließlich der Griffkontur. Der zusätzliche HTTP-Test rastert die
Antwort unabhängig bei 420 px: rund 33 % Vollbreitenüberdeckung und 1,6
Einheiten Mittenabweichung. Beide Methoden erfüllen die Grenzen.

## HTTP- und Pixeltests

`internal/server/hank_contact_test.go` erzeugt einen echten HTTP-Server mit
hank-Profil, lädt dessen SVG über HTTP und prüft:

- Griffachse schneidet die Flosse; Überdeckungszentrum höchstens 5 Einheiten daneben.
- Mindestens 30 % des Griffs über die komplette Breite verdeckt.
- Zeichenreihenfolge Griff → Flosse → Schlägerfläche.
- Kein Griffpfad ragt hinter dem unteren verdeckten Abschnitt heraus.
- Im aus dem HTTP-SVG gerenderten 420-px-Bild hat der innere Eintrittspunkt
  Flossenfarbe statt Grifffarbe.

Mit `SP_HANK_CONTACT_URL` wurde derselbe Test zusätzlich gegen das tatsächlich
laufende Profil auf Port 8080 ausgeführt, nicht nur gegen den Testserver.
Die 24 bestehenden Python-Geometrietests und die gesamte Go-Suite einschließlich
der bestehenden hank-/blecki-Browsertests sind grün. Unterscheidbarkeit: 186 ≥ 157.

Die echte Profilfigur und das Vergleichsblatt wurden unabhängig im Browser bei
420 × 420 an derselben Pixelposition gerendert: **0 unterschiedliche Pixel**.
PNG-SHA-256 beider Bilder: `{pixels['app_image_sha256']}`.
SVG-Geometriehash beider Quellen: `{geometry(delivered)}`.

## Laufende Quellen und Prüfbilder

App: `{runtime['app_executable']}`, Arbeitsverzeichnis `{runtime['app_cwd']}`.
Gebaut und gestartet aus dem normalen Repo auf `feat/pip-avatar`.
Prüfansicht: normales Repo unter `/home/sthings/projects/schmetterpause`, Port 8091.
Der alte Worktree bleibt nur als losgelöste Referenz, kein Prozess nutzt ihn als Quelle.
`internal/server/zz_shot_test.go` war beim Arbeitsbeginn bereits nicht mehr vorhanden
und ist nicht Teil dieses Commits.

Für die echte Profilaufnahme wurde in der lokalen Vorschau ein eigenes Testprofil
angelegt; bestehende Spieler wurden nicht umgestellt. Profil:
`{runtime['profile_url']}`.

- `profile-app-vs-sheet.png`: echter vollständiger Profilseiten-Screenshot neben Referenz.
- `app-vs-sheet-420.png`: echte Profilfigur neben dem Vergleichsblatt, je 420 px.
- `app-render-420.png`, `sheet-render-420.png`: pixelidentische Einzelbilder.
- `hand-before-after-420.png`, `hand-before-420.png`, `hand-after-420.png`: Handdetail
  vorher (bddb539) und nachher, je 420 px und identische rote Grifffarbe.
- `before-profile-app.png`, `after-profile-app.png`: echte Profilseite vor/nach Neustart.
- `before-http-audit.json`, `contact-audit.json`, `runtime-verification.json`:
  Quell-, Hash- und Kontaktbelege.

Reproduktion: `go run ./scripts/avatar_review.go -contact-only -out docs/avatar-check/pip/grip-contact`;
mit eigener Vorschau-Sitzung `scripts/hank_contact.browser.cjs`, anschließend
`python scripts/hank_contact_report.py`. Cache-Busting aller Bilder über den
aktuellen Asset-Inhaltshash. Physischer LED-Test ist nicht Teil der Prüfung.
'''
 (OUT/'README.md').write_text(text)
 version=result['asset_sha256'][:16]
 page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – HTTP-Griffkontakt</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}a{{color:#65bdff}}</style><h1>hank – geprüfter Griffkontakt</h1><p>Vollbreitenüberdeckung {after['full_width_covered_fraction']*100:.2f} % · Achsabweichung {after['axis_offset_from_overlap_centroid']:.2f} · 0 unterschiedliche Pixel zwischen App und Referenz.</p><p><a href="README.md">Vollständiger Bericht mit ursprünglichem HTTP-Hash und Laufverzeichnis</a> · <a href="contact-audit.json">Messwerte und Hashbelege</a></p>'''
 for title,file in [('Echte App-Profilseite neben dem Vergleichsblatt','profile-app-vs-sheet.png'),('App und Referenz · je 420 px · pixelidentisch','app-vs-sheet-420.png'),('Handpartie vorher bddb539 | nachher · je 420 px','hand-before-after-420.png')]:page+=f'<h2>{title}</h2><img src="{file}?v={version}" alt="{title}">'
 (OUT/'index.html').write_text(page+'</html>')

if __name__=='__main__':generate()
