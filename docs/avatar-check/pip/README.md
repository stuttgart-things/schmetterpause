# hank – freigegebener Fäustling als Produktstand

Die freigegebene Fäustling-Variante ersetzt die angedrückte Flosse vollständig.
Basis und Schläger-SVG sind **bytegleich mit dem freigegebenen Entwurf**.
Kein Daumen, keine einzelnen Finger; eine geschlossene runde Faust in
Flossenfarbe und höchstens eine geschwungene Falzlinie. Ein breiter Arm verbindet
die Schulter ohne dünnes Handgelenk mit der Faust. Der ungeteilte Stiel liegt
hinter der Faust und ist darüber und darunter einschließlich Kappe sichtbar.

## Messwerte an den Produkt-SVGs

| Messung | Produkt |
|---|---:|
| Faust-/Stielbreite einschließlich Kontur | 58.000 / 36.011 = **1.611** |
| Verhältnis der Farbflächen | **1.692** (zulässig 1,5–1,8) |
| Sichtbarer Hals bis zur Faustkontur | **20.9 ≥ 20** |
| Unteres sichtbares Stielstück einschließlich Kappe | **11.0 ≥ 10** |
| Gemessene Stielachse | 13.429° (Vorlage 13,42°) |
| Abweichung der getrennt gemessenen oberen/unteren Zentren | **0.048° ≤ 2°** |
| Vollständige Querüberdeckung | 18.25 Einheiten Stiellänge |
| Farbflächenüberlappung Arm / Faust | 628.65 Einheiten² |
| Abstand Hand-/Armpartie zum Emblem | **17.027 ≥ 12** |
| Abstand Hand-/Armpartie zum Canvas | **13.265 ≥ 8** |
| Canvas-Abstand einschließlich unveränderter Schlägerfläche | 8.250 ≥ 8 |
| Unterscheidbarkeit im quantisierten 64-px-GIF | **184 ≥ 157** |

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

Echte Profilfigur auf Port 8080 und Referenz: **0 unterschiedliche Pixel bei 420 × 420**. PNG-SHA-256 beider Bilder: `5337e13b317c62c7c5474bb15362a7468d5e459fa2d89cae3fb5bdb0da3b2365`.

App und Prüfansicht werden aus `/home/sthings/projects/schmetterpause` gestartet,
nicht aus einem separaten Worktree. `runtime-verification.json` dokumentiert die
laufenden Quellen. Bild-Cachekennung: `90991129e78f99c7`.

## Dateien und Prüfbilder

- Produkt: `web/static/img/pip/base.svg`, `web/static/img/pip/racket.svg`.
- Generator: `scripts/hank_mitten.py`, `scripts/import_creator_avatars.py`;
  `scripts/hank_press.py` ist entfernt.
- Messung: `scripts/hank_flipper_geometry.py`.
- Tests: `scripts/test_hank_{axis,emblem,racket}_geometry.py`,
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
