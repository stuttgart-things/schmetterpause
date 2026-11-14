# hank – beide Flossen in derselben Formensprache

Vergleich: **1298bfe → aktuelles Artwork**. Bildversion: `1298bfedc6272671bd756f5a6b00980960082a55-155b47d05bf8`.

Die rechte Flosse ist eine breite, durchgehende Form von der Schulter bis zur
abgerundeten, quer über den Griff gebogenen Spitze. Keine Finger oder Daumen;
nur eine einzelne gefüllte Falzlinie. Die Spitze liegt in `racket` vor dem Griff.
Die Flosse in `base` und die vordere Flosse haben dieselbe Geometrie und überlappen
vollständig: kein dünnes Handgelenk und keine Naht.

## Messwerte aus den erzeugten SVG-Pfaden

| Messung | Wert |
|---|---:|
| Schmalster vergleichbarer Flossenquerschnitt links | 49.40 |
| Schmalster vergleichbarer Flossenquerschnitt rechts | 50.30 |
| Rechts / links | 101.82 % (zulässig 85–115 %) |
| Rechte Kontur, Abstand zur Farbfläche | 7 Einheiten |
| Abstand Flosse / Emblem | 14.004 (mindestens 12) |
| Kleinster Canvas-Abstand der Flosse | 8.674 (mindestens 8) |
| Sichtbare Grifffarbe, abzüglich Streifen und Überdeckung | 621.69 Einheiten² (mindestens 540) |
| Freier Griffhals | 21.50 |
| Unterer sichtbarer Griffabschnitt | 8.00 |
| Griffachse | 13.42057°, unverändert |
| Unterscheidbarkeit im quantisierten 64-px-GIF | 186 (mindestens 157) |

Die Breitenmessung folgt der jeweiligen aus den gefüllten Pfaden gemessenen
Längsachse. Der vergleichbare Schaftbereich reicht von 25 bis 65 % der Länge;
Schulteranschluss und runde Endkappe sind ausgeschlossen. Alle 81 Querschnitte
pro Seite stehen in `measurements.json`. Die Messung überspannt die Falzöffnung.
Oberer und unterer Griffabschnitt liegen auf derselben Achse; die Spitze
überdeckt die volle Griffbreite. Ein Test sichert diese Querüberdeckung separat.

## Prüfbilder

- `both-flippers-before-420.png`: links und rechts, je 420 px; Stand 1298bfe.
- `both-flippers-after-420.png`: dieselben Ausschnitte nachher, je 420 px.
- `left-flipper-{before,after}-420.png`, `right-flipper-{before,after}-420.png`: Einzelbilder.
- `expressions-192-app.png`: sechs Ausdrücke × drei Grundfarben, 192 px auf Navy.
- `expressions-initial-192-app.png`: dieselben 18 Varianten mit Initiale A.

## Änderungen und unveränderte Bereiche

Produktassets: nur `web/static/img/pip/base.svg` (zwei Pfade der rechten Flosse)
und `web/static/img/pip/racket.svg` (Flossengruppe).
Generator: `scripts/hank_pose.py`, `scripts/import_creator_avatars.py`.
Messung/Tests: `scripts/hank_flipper_geometry.py`, `scripts/test_hank_*geometry.py`,
SVG-Strukturprüfungen in `internal/avatar/{avatar,pip}_test.go`.
Prüfbilder/Seite: `scripts/avatar_review.go`, `scripts/hank_completion_review.py`,
`docs/avatar-check/pip/`.

`invariants-1298bfe.json` sichert alle übrigen SVGs bytegenau, einschließlich
Augen, Schnäbeln, Emblem, Stirnband und anderen Avataren. Der Basistest erlaubt
nur die beiden rechten Flossenpfade; die linke Flosse bleibt bytegenau gleich.
Schlägerfläche, kompletter Griff einschließlich Streifen, Position und Achse
bleiben bytegenau gleich. Paletten, Rahmung und Glyphen bleiben unverändert.

Reproduktion mit Python samt fontTools/skia-pathops:
`python scripts/hank_completion_review.py --serve 8091`.
HTTP sendet `Cache-Control: no-store`; alle Bild-URLs enthalten Quellcommit und
SVG-Inhaltshash. Physischer LED-Test und optische Abnahme stehen aus.
