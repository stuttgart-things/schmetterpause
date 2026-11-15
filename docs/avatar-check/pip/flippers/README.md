# hank – ausgewählte Produktvariante B, 80 %

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
| Gewählte lineare Flossengröße | 80 % |
| Tatsächliche Flossenbreite rechts / links | 80.000 % |
| Breite links / rechts | 94.291 / 75.433 |
| Emblemabstand einschließlich Schulter | 12.541 ≥ 12 |
| Canvas-Abstand | 8.070 ≥ 8 |
| Freier Griffhals über der Flosse | 20.70 ≥ 20 |
| Sichtbare Grifffarbe ohne Streifen/Überdeckung | 477.046 Einheiten² |
| Überdeckung von Gesicht / Schlägerfläche | 0.000 / 0.000 |
| Unterscheidbarkeit im quantisierten 64-px-GIF | 186 ≥ 157 |
| Schwächstes Paar | neutral/skeptical |

Die angelegte Lage wurde für die größere Flosse angepasst: gespiegelte native
Kontur, Drehung -180°, ohne Änderungen an Gesicht, Schlägerfläche,
Schlägerachse, Rahmung oder Emblem. Die gesamte Basis einschließlich Schulter
ist bytegleich mit 1298bfe. Der engste Abstand ist unten am Canvas (8,07);
mit 80 % wird der angefragte Größenbereich vollständig ausgeschöpft.
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
`scripts/test_hank_{axis,emblem,racket}_geometry.py`,
`internal/avatar/pip_test.go`, `internal/server/static_avatar_test.go`.
Cache: `internal/server/static.go`, `web/static/js/live.js`.

Unter `docs/avatar-check/pip/hand-variants/`:

- `comparison-192-navy.png`: sthings | 1298bfe | B80, Neutral/Fröhlich/Wütend,
  zuerst ohne, danach mit Initiale A (192 px auf Navy).
- `hand-racket-comparison-420.png`: dieselben Spalten je 420 px, roter Griff.
- `B-neutral-64-black.png`, `B-neutral-64.gif`: native 64 px.
- `B-{face}.svg`: vollständige Renderings aller sechs Ausdrücke.
- `measurements-B.json`, `separation.json`: Messdaten und quantisierte Trennung.

Unter `docs/avatar-check/pip/flippers/` zusätzlich die 18er-Matrizen aller sechs
Ausdrücke × drei Grundfarben, ohne und mit Initiale A.

Bildversion: `4fce46052dd9d96cd86c33c199d3dc4d08aeb14a-68232cbea217efdf`. Prüfbild-URLs enthalten Commit- und Inhaltshash.
Die laufende App lädt Live-Avatare mit Cache-Buster; ihre pip-Assets müssen
revalidiert werden. Die üblichen eingebetteten Profilfiguren werden beim
Neubau der App aktualisiert.

Reproduktion: Vector-Tools-Python mit fontTools/skia-pathops,
`python scripts/hank_completion_review.py --serve 8091`.
Produktimport wie bisher mit `scripts/import_creator_avatars.py`.
Physischer LED-Test und optische Abnahme wurden nicht vorweggenommen.
