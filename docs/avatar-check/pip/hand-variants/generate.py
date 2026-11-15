#!/usr/bin/env python3
"""Render selected product B against 1298bfe. Does not modify product assets."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
ROOT=Path(__file__).resolve().parents[4]
OUT=Path(__file__).resolve().parent
sys.path.insert(0,str(ROOT/'scripts'))
from hank_flipper_geometry import measurements

def generate():
    data=(ROOT/'web/static/img/pip/racket.svg').read_bytes()
    (OUT/'B-racket.svg').write_bytes(data)
    for part in ('base','racket'):
        (OUT/('baseline-'+part+'.svg')).write_bytes(subprocess.check_output(['git','show','1298bfe:web/static/img/pip/'+part+'.svg'],cwd=ROOT))
    assert (OUT/'baseline-base.svg').read_bytes()==(ROOT/'web/static/img/pip/base.svg').read_bytes()
    subprocess.run(['go','run','./docs/avatar-check/pip/hand-variants/render.go'],cwd=ROOT,check=True)
    m=measurements();m.update(json.loads((OUT/'separation.json').read_text()))
    assert .75<=m['flipper_width_ratio']<=.81
    assert m['emblem_clearance']>=12 and m['canvas_margin']>=8-.001 and m['free_neck_length']>=20
    assert m['face_overlap_area']<.001 and m['blade_overlap_area']<.001 and m['minimum_separation']>=157
    revision=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
    version=revision+'-'+hashlib.sha256(data).hexdigest()[:16]
    m['version']=version;m['status']='selected product B';m['maximum_requested_scale']=.8
    (OUT/'measurements-B.json').write_text(json.dumps(m,indent=2)+'\n')
    report=f'''# hank – ausgewählte Produktvariante B, 80 %

B drückt den Schläger mit der angelegten Flosse an. Die Griffteile werden zuerst
gezeichnet, danach die Flosse, zuletzt die unveränderte Schlägerfläche vor dem
Gesicht. Der unsichtbare Griffabschluss liegt vollständig hinter der Flosse.
Keine Daumen-/Fingerformen. Die natürliche einzelne Falz der linken Flosse wird
mitgespiegelt. A und C werden nicht benötigt.

## Größe und Messwerte

**80 %** sind der größte Wert im angefragten Bereich 75–80 %. Dieser obere
Grenzwert erfüllt alle Bedingungen; eine kleinere Flosse ist nicht nötig.
Die gleichmäßige Skalierung wird sowohl über die Fläche als auch unabhängig
über die Kurvenbreite quer zur gemessenen Längsachse geprüft.

| Messung | Produkt B |
|---|---:|
| Gewählte lineare Flossengröße | {m['selected_scale']*100:.0f} % |
| Tatsächliche Flossenbreite rechts / links | {m['flipper_width_ratio']*100:.3f} % |
| Breite links / rechts | {m['left_width']:.3f} / {m['right_width']:.3f} |
| Emblemabstand einschließlich Schulter | {m['emblem_clearance']:.3f} ≥ 12 |
| Canvas-Abstand | {m['canvas_margin']:.3f} ≥ 8 |
| Freier Griffhals über der Flosse | {m['free_neck_length']:.2f} ≥ 20 |
| Vollständig überdeckte Grifflänge | {m['full_width_covered_fraction']*100:.3f} % ≥ 30 % |
| Abstand Überdeckungsmittelpunkt / Griffachse | {m['axis_offset_from_overlap_centroid']:.3f} ≤ 5 |
| Sichtbare Grifffarbe ohne Streifen/Überdeckung | {m['visible_grip_area']:.3f} Einheiten² |
| Überdeckung von Gesicht / Schlägerfläche | {m['face_overlap_area']:.3f} / {m['blade_overlap_area']:.3f} |
| Unterscheidbarkeit im quantisierten 64-px-GIF | {m['minimum_separation']} ≥ 157 |
| Schwächstes Paar | {m['closest_pair']} |

Die angelegte Lage wurde für die größere Flosse angepasst: gespiegelte native
Kontur, Drehung {m['angle_degrees']}°, ohne Änderungen an Gesicht, Schlägerfläche,
Schlägerachse, Rahmung oder Emblem. Die gesamte Basis einschließlich Schulter
ist bytegleich mit 1298bfe. Der engste Abstand ist unten am Canvas (8,07);
mit 80 % wird der angefragte Größenbereich vollständig ausgeschöpft.
Seit der Kontaktprüfung läuft der versteckte Griff tiefer in die Flosse;
mindestens 30 % seiner Länge sind über die volle Breite verdeckt. Der
Kontaktmittelpunkt liegt höchstens 5 Einheiten neben der Achse.
Die alte Umgreif-Prüfung mit unterem Griffende entfällt: B hält durch Andrücken.
Die Prüfung verlangt stattdessen mindestens 20 Einheiten Griffhals, Griff hinter
der Flosse, einen vollständig versteckten Griffabschluss, mindestens 75 %
Flossenbreite sowie keine Überdeckung von Gesicht oder Schlägerfläche.

## Dateien und Prüfbilder

Produkt: `web/static/img/pip/base.svg` (Schulter wieder 1298bfe),
`web/static/img/pip/racket.svg` (B vor gekürztem Griff).
Generator: `scripts/hank_press.py`, `scripts/hank_pose.py`,
`scripts/import_creator_avatars.py`.
Messung/Tests: `scripts/hank_flipper_geometry.py`,
`scripts/test_hank_{{axis,emblem,racket}}_geometry.py`,
`internal/avatar/pip_test.go`, `internal/server/static_avatar_test.go`.
Cache: `internal/server/static.go`, `web/static/js/live.js`.

Unter `docs/avatar-check/pip/hand-variants/`:

- `comparison-192-navy.png`: sthings | 1298bfe | B80, Neutral/Fröhlich/Wütend,
  zuerst ohne, danach mit Initiale A (192 px auf Navy).
- `hand-racket-comparison-420.png`: dieselben Spalten je 420 px, roter Griff.
- `B-neutral-64-black.png`, `B-neutral-64.gif`: native 64 px.
- `B-{{face}}.svg`: vollständige Renderings aller sechs Ausdrücke.
- `measurements-B.json`, `separation.json`: Messdaten und quantisierte Trennung.

Unter `docs/avatar-check/pip/flippers/` zusätzlich die 18er-Matrizen aller sechs
Ausdrücke × drei Grundfarben, ohne und mit Initiale A.

Bildversion: `{version}`. Prüfbild-URLs enthalten Commit- und Inhaltshash.
Die laufende App lädt Live-Avatare mit Cache-Buster; ihre pip-Assets müssen
revalidiert werden. Die üblichen eingebetteten Profilfiguren werden beim
Neubau der App aktualisiert.

Reproduktion: Vector-Tools-Python mit fontTools/skia-pathops,
`python scripts/hank_completion_review.py --serve 8091`.
Produktimport wie bisher mit `scripts/import_creator_avatars.py`.
Physischer LED-Test und optische Abnahme wurden nicht vorweggenommen.
'''
    (OUT/'README.md').write_text(report)
    page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – Produkt B 80 %</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}a{{color:#65bdff}}.pixel{{image-rendering:pixelated}}</style><h1>hank – Produkt B · 80 %</h1><p>Emblem {m['emblem_clearance']:.2f} · Canvas {m['canvas_margin']:.2f} · Griffhals {m['free_neck_length']:.1f} · Unterscheidbarkeit {m['minimum_separation']} ≥ 157.</p><p><a href="README.md">Bericht und geänderte Dateien</a> · <a href="measurements-B.json">Messwerte</a> · <a href="../index.html">Alle sechs Ausdrücke und Grundfarben</a></p>'''
    for title,filename in [('sthings | 1298bfe | B80 · 192 px','comparison-192-navy.png'),('Schläger-/Flossenpartie · je 420 px','hand-racket-comparison-420.png'),('Neutral · native 64 px auf Schwarz','B-neutral-64-black.png')]:
        page+=f'<h2>{title}</h2><img src="{filename}?v={version}" alt="{title}">'
    (OUT/'index.html').write_text(page+'</html>')
    print(json.dumps(m,indent=2))
    return m,report,version

if __name__=='__main__':generate()
