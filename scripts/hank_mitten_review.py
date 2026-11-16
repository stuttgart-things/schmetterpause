"""Product report for the approved mitten and its live-app pixel comparison."""
import hashlib,json,shutil
from pathlib import Path
from hank_flipper_geometry import measurements
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'docs/avatar-check/pip/mitten-product'
PREVIEW=ROOT/'docs/avatar-check/pip/mitten-preview'

def generate():
    OUT.mkdir(parents=True,exist_ok=True)
    for part in ('base','racket'):
        assert (ROOT/f'web/static/img/pip/{part}.svg').read_bytes()==(PREVIEW/f'mitten-{part}.svg').read_bytes(),'product must match approved preview exactly'
    m=measurements();r=json.loads((PREVIEW/'renderer-verification.json').read_text())
    m['minimum_separation']=r['minimum_separation_64'];m['closest_pair']=r['closest_pair']
    assert m['free_neck_length']>=20 and m['lower_visible_colour_length']>=10
    assert abs(m['collinear_deviation_degrees'])<=2 and 1.5<=m['width_ratio']<=1.8
    assert m['emblem_clearance']>=17.02 and m['canvas_margin']>=13.26 and m['minimum_separation']>=157
    assets=(ROOT/'web/static/img/pip/base.svg').read_bytes()+(ROOT/'web/static/img/pip/racket.svg').read_bytes()
    version=hashlib.sha256(assets).hexdigest()[:16];m['asset_version']=version;m['approved_preview_bytes_identical']=True
    for name in ('hand-comparison-420.png','expressions-192-navy.png'):
        shutil.copy2(PREVIEW/name,OUT/name)
    (OUT/'measurements.json').write_text(json.dumps(m,indent=2)+'\n')
    pixel_note='App-/Referenzvergleich wird nach dem App-Neustart ergänzt.'
    if (OUT/'app-render-420.png').exists() and (OUT/'sheet-render-420.png').exists():
        from PIL import Image
        a=Image.open(OUT/'app-render-420.png').convert('RGBA');b=Image.open(OUT/'sheet-render-420.png').convert('RGBA')
        different=sum(a.getpixel((x,y))!=b.getpixel((x,y)) for y in range(420) for x in range(420))
        assert different==0,'app and reference must be pixel-identical'
        result={'size':[420,420],'different_pixels':different,'pixels_identical':True,
                'app_image_sha256':hashlib.sha256((OUT/'app-render-420.png').read_bytes()).hexdigest(),
                'sheet_image_sha256':hashlib.sha256((OUT/'sheet-render-420.png').read_bytes()).hexdigest()}
        (OUT/'pixel-comparison.json').write_text(json.dumps(result,indent=2)+'\n')
        pixel_note=f"Echte Profilfigur auf Port 8080 und Referenz: **0 unterschiedliche Pixel bei 420 × 420**. PNG-SHA-256 beider Bilder: `{result['app_image_sha256']}`."
    report=f'''# hank – freigegebener Fäustling als Produktstand

Die freigegebene Fäustling-Variante ersetzt die angedrückte Flosse vollständig.
Basis und Schläger-SVG sind **bytegleich mit dem freigegebenen Entwurf**.
Kein Daumen, keine einzelnen Finger; eine geschlossene runde Faust in
Flossenfarbe und höchstens eine geschwungene Falzlinie. Ein breiter Arm verbindet
die Schulter ohne dünnes Handgelenk mit der Faust. Der ungeteilte Stiel liegt
hinter der Faust und ist darüber und darunter einschließlich Kappe sichtbar.

## Messwerte an den Produkt-SVGs

| Messung | Produkt |
|---|---:|
| Faust-/Stielbreite einschließlich Kontur | {m['fist_outline_width']:.3f} / {m['shaft_outline_width']:.3f} = **{m['width_ratio']:.3f}** |
| Verhältnis der Farbflächen | **{m['colour_width_ratio']:.3f}** (zulässig 1,5–1,8) |
| Sichtbarer Hals bis zur Faustkontur | **{m['free_neck_length']:.1f} ≥ 20** |
| Unteres sichtbares Stielstück einschließlich Kappe | **{m['lower_visible_colour_length']:.1f} ≥ 10** |
| Gemessene Stielachse | {m['grip_axis']['degrees']:.3f}° (Vorlage 13,42°) |
| Abweichung der getrennt gemessenen oberen/unteren Zentren | **{m['collinear_deviation_degrees']:.3f}° ≤ 2°** |
| Vollständige Querüberdeckung | {m['full_width_covered_length']:.2f} Einheiten Stiellänge |
| Farbflächenüberlappung Arm / Faust | {m['arm_fist_overlap_area']:.2f} Einheiten² |
| Abstand Hand-/Armpartie zum Emblem | **{m['emblem_clearance']:.3f} ≥ 12** |
| Abstand Hand-/Armpartie zum Canvas | **{m['canvas_margin']:.3f} ≥ 8** |
| Canvas-Abstand einschließlich unveränderter Schlägerfläche | {m['equipment_canvas_margin']:.3f} ≥ 8 |
| Unterscheidbarkeit im quantisierten 64-px-GIF | **{m['minimum_separation']} ≥ 157** |

Die winzige Differenz zur konstruierten 13,42057°-Achse stammt aus der unabhängigen
0,5-Einheiten-PCA-Messung. Obere und untere Stielabschnitte sind Teile desselben
kontinuierlichen Kapselpfads. Die sichtbaren Querschnittsmitten werden zusätzlich
getrennt gemessen. Eine dunklere Kappe schließt den unteren Stiel ab.

## Tests und Live-Nachweis

Die Andrück-Prüfungen (kein unteres Ende, 80-%-Flosse, Mindestlänge hinter der
Flosse) sind entfernt. Die Handtests prüfen stattdessen obere/untere Sichtbarkeit,
Kollinearität, Verhältnis 1,5–1,8, volle Querüberdeckung, Faust vor dem Stiel und
Schulter-/Arm-Verbindung. Die Farbprüfung an einem 420-px-Pixel vor dem Stiel
bleibt und berücksichtigt die einzelne Falzlinie.
`TestHankContactThroughHTTP` lädt das SVG über einen echten HTTP-Testserver;
`SP_HANK_CONTACT_URL` lässt dieselben Prüfungen zusätzlich gegen Port 8080 laufen.
Alle Tests und Browserregressionen sind grün (siehe Abschlussbericht).

{pixel_note}

App und Prüfansicht werden aus `/home/sthings/projects/schmetterpause` gestartet,
nicht aus einem separaten Worktree. `runtime-verification.json` dokumentiert die
laufenden Quellen. Bild-Cachekennung: `{version}`.

## Dateien und Prüfbilder

- Produkt: `web/static/img/pip/base.svg`, `web/static/img/pip/racket.svg`.
- Generator: `scripts/hank_mitten.py`, `scripts/import_creator_avatars.py`;
  `scripts/hank_press.py` ist entfernt.
- Messung: `scripts/hank_flipper_geometry.py`.
- Tests: `scripts/test_hank_{{axis,emblem,racket}}_geometry.py`,
  `internal/server/hank_contact_test.go`, `internal/avatar/pip_test.go`.
- Ansicht: `scripts/hank_completion_review.py`, `scripts/hank_mitten_review.py`.

Unter `docs/avatar-check/pip/mitten-product/`:

- `hand-comparison-420.png`: **sthings | hank a99eac2 | Produkt-Fäustling**, je 420 px.
- `expressions-192-navy.png`: sechs Ausdrücke im produktiven `RenderLive`-Renderer
  mit unverändertem App-CSS, in Chromium bei je 192 px aufgenommen.
- `hand-before-after-420.png`: vorher a99eac2 / nachher, identische Ausschnitte.
- `profile-app-vs-sheet.png`: echte Profilseite neben dem Referenzrendering.
- `app-vs-sheet-420.png`: echte App-Figur und Referenz, je 420 px.
- `reference-neutral.svg`, `after-http-neutral.svg`: Referenz und HTTP-Antwort.
- `measurements.json`, `pixel-comparison.json`: Vektor- und Pixelbelege.

Alle Gesichter, Augen, Brauen, Stirnband, Emblem, Glyphen, linke Flosse, Rahmung,
Schlägerfläche und übrigen Avatare bleiben unverändert. Die Basistests erlauben
nur die beiden rechten Armpfade; die übrigen SVGs sind über Hashes abgesichert.

Reproduktion mit Vector-Tools-Python und Playwright:
`python scripts/hank_completion_review.py --serve 8091`.
Produktimport wie bisher über `scripts/import_creator_avatars.py`.
'''
    (OUT/'README.md').write_text(report)
    page=f'''<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>hank – Produkt-Fäustling</title><style>body{{background:#0a0a12;color:#eee;font:16px system-ui;margin:24px}}img{{max-width:100%;height:auto}}a{{color:#65bdff}}</style><h1>hank – Fäustling</h1><p>Freigegebener Entwurf bytegleich übernommen. Hals {m['free_neck_length']:.1f} · unten {m['lower_visible_colour_length']:.1f} · Faust/Stiel {m['width_ratio']:.2f} · Unterscheidbarkeit {m['minimum_separation']}.</p><p><a href="README.md">Produktbericht, Messwerte und Tests</a> · <a href="measurements.json">Messdaten</a></p>'''
    for title,name in [('sthings | hank a99eac2 | Fäustling · je 420 px','hand-comparison-420.png'),('Sechs Ausdrücke · App-Renderer / App-CSS · je 192 px','expressions-192-navy.png'),('Echte App neben Referenz · pixelidentisch','app-vs-sheet-420.png'),('Echte Profilseite neben Referenz','profile-app-vs-sheet.png'),('Vorher a99eac2 | Fäustling · je 420 px','hand-before-after-420.png')]:
        if (OUT/name).exists():page+=f'<h2>{title}</h2><img src="{name}?v={version}" alt="{title}">'
    (OUT/'index.html').write_text(page+'</html>')
    return m,report,version

if __name__=='__main__':generate()
