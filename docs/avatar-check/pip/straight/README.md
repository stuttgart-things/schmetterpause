# hank – gerader Schläger und sichtbare Hand

Referenz: 8822413. Rahmung 110 %, Schlägerfläche, alle sechs Gesichter, Basis, Emblem, Paletten und andere Avatare bleiben unverändert. `invariants-8822413.json` und der Geometrietest sichern die ausgeschlossenen SVGs und Glyphen bytegenau ab. Die beiden Flächenpfade sind exakt gleich geblieben.

## Messwerte (420 × 420)

| Messung | sthings | hank vorher | hank nachher |
|---|---:|---:|---:|
| Griffachse zur Senkrechten | 14,34° | 39,62° | 13,42° |
| Gesamte funktionale Schlägerachse | 13,94° | – | 13,43° |
| Flächenachse aus unterem Randnormal | 15,59° | 15,59° | 15,59° |
| Abweichung Fläche/Griff | – | 24,03° | 2,17° |
| Seitlicher Abstand Flächenschwerpunkt/Griffachse | – | 29,66 | 0,02 |
| Freier Griffhals einschließlich Streifen | 14,6 | 5,5 | 25,1 |
| Farbiges Griffende unter der Hand | 5,7 | 29,2 | 10,4 |
| Sichtbare gewählte Grifffarbe, Fläche | – | 861,92 | 751,35 |

Die sthings-Pfote liegt mit ihrem Bounding-Box-Mittelpunkt 14,29 Einheiten links und 137,36 unter dem Flächenschwerpunkt; ihre Farbbox ist (294,313)–(376,418), einschließlich Handgelenk. Entlang der Griffachse sind oberhalb 13,4 und unterhalb 5,7 Einheiten Griff-Farbe sichtbar. Bei hank unterbrechen die Streifen den 25,1 langen freien Hals; die reine Farbe darin ist 21,6 lang. Die neue Hand umfasst den Griff von vorn und ist vollständig sichtbar, inklusive Verbindung zum vorhandenen Arm.

Methodik: Flächenschwerpunkt aus gefüllten Polygonen. Funktionale Referenzachse vom Flächenschwerpunkt zum oberen dominanten Grifffeld; gesamte Achse zum unteren Feld. Die unabhängige geometrische Flächenachse ist die Normale des längsten unteren Randsegments. Die Griffachse wird unabhängig von den Generator-Konstanten aus im gefüllten Griff gesampelten Punkten bestimmt. Die fast runde Fläche allein liefert keine stabile PCA-Längsachse. Vollständige Daten: `axis-measurements.json`, `pose-measurements.json`, `measurements.json` und `raster-measurements.json`.

Kleinster Canvas-Abstand: 8,25. Emblem-Abstand zur Ausrüstung/Flosse: 40,48. Kleinster Augenabstand weiterhin 6,23. Der Test der tatsächlichen quantisierten 64-px-GIFs findet bei allen sechs Ausdrücken keine direkte oder diagonale Berührung dunkler Augen- und Schlägerpixel. Deshalb wurde die Fläche nicht verschoben. Griffstreifen stehen quer zur Achse; `racket` bleibt die letzte Gruppe.

## Prüfbilder

- `sthings-hank-192-app.png`: alle sechs Ausdrücke, ohne Schmuck.
- `sthings-hank-emblem-192-app.png`: gleicher Vergleich mit Initiale A.
- `hand-racket-comparison-420.png`: sthings | hank vorher | hank nachher, identischer Ausschnitt, jeweils 420 px. Nur hier ist der vorhandene rote Griff zur besseren Sichtbarkeit gewählt.
- `../expressions-row-64-black.png` und `../expressions-row-28-black.png`: sechs Ausdrücke auf Schwarz.
- `../expressions-{192,64,28}-{app,black}.png`: alle drei Grundfarben und sechs Ausdrücke.
- `selection-app.png`: tatsächliche Figurenauswahl aus dem Browsertest.

## Geänderte Dateien und Prüfung

Produktasset: nur `web/static/img/pip/racket.svg`. Generator: `scripts/hank_pose.py`, `scripts/import_creator_avatars.py`, neue gemeinsame Achsmessung `scripts/racket_axis.py`. Prüfbilder/Messung: `scripts/avatar_review.go`, `scripts/measure_hank_holding.py`, `scripts/measure_hank_pose.py`, `scripts/measure_hank_straight.py`. Tests: `scripts/test_hank_axis_geometry.py`, `internal/avatar/hank_axis_test.go`. Dokumentation und generierte Prüfbilder unter `docs/avatar-check/pip/` wurden aktualisiert.

21 Python-Geometrietests, `go test ./...` und der bestehende Playwright-Browsertest sind grün. Der neue Go-Test prüft Augen/Schläger-Trennung im tatsächlichen 64-px-GIF. Unterscheidbarkeit: **193**, schwächstes Paar **Skeptisch/Wütend**, unverändert; Mindestwert 157. Emblemvarianten ohne Name, Anna und Ümit ebenfalls 193. Verbotene SVG-Elemente/Attribute werden durch bestehende Tests ausgeschlossen.

Reproduktion: `scripts/measure_hank_straight.py`, `scripts/measure_hank_pose.py` mit der vorhandenen Vector-Tools-Python-Umgebung; anschließend `go run ./scripts/avatar_review.go -straight-only -out docs/avatar-check/pip/straight`. Allgemeine Bilder: `go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip`.

Offen bleiben die optische Abnahme und ein Foto vom echten LED-Panel. Die Bildschirm-/GIF-Prüfung ersetzt keinen Hardwaretest.
