# hank – engerer Bildausschnitt

Der freigegebene Kompromiss priorisiert Kopfbreite und Rumpfanschnitt. Kopf und
Stirnband behalten ihre Form; die Augenlinie wird nicht separat verschoben.

| Vorlage | Kopfbreite bei 420 px | Anteil | Augenlinie | Oberer Rand | Unterer Figurenrand |
|---|---:|---:|---:|---:|---:|
| sthings | 383 px | 91,2 % | 181,5 / 43,2 % | 2 px | 418 / 99,5 % |
| hank vorher | 316 px | 75,2 % | 225,4 / 53,7 % | 11 px | 420 / 100 % |
| hank nachher | 379 px | 90,2 % | 241,7 / 57,6 % | 8 px | 420 / 100 % |
| blecki, unverändert | 296 px | 70,5 % | 201,5 / 48,0 % | 15 px | 420 / 100 % |

Die Kopfmasken werden bei 420 px gerendert und bei 50 % Deckung ausgemessen.
Ohren/Kopfschmuck gehören zur Breite, Schläger nicht. Der schmale Federschopf
verändert die maximale Kopfbreite nicht. Die Augenlinie ist der Mittelwert der
Neutral-Augenmittelpunkte (blecki: bestehende Augenanker). Die Masken und ihre
Definitionen sind abgelegt, damit die Messung reproduzierbar bleibt.

Zentraler Schritt: `scripts/hank_frame.py`, Skalierung **1,2**, Verschiebung
**(-27,21127; -28,75816)**, in alle Pfade eingerechnet. Der Schopf oberhalb y=60
wird vorher ausschließlich vertikal um **40 %** gekürzt. Alle Spitzen bleiben
vorhanden. Nach anschließender 120-%-Rahmung ist seine sichtbare Höhe gegenüber
vorher effektiv 28 % kleiner. Kopf und Stirnband unterhalb dieser Grenze werden
nur gleichmäßig skaliert und verschoben.

Der neue untere Canvas-Schnitt entspricht y=373,97 der alten Figur statt y=420.
Unter dem Kinn bleiben etwa 70,8 statt 105 Einheiten sichtbar. Bei sthings liegen
zwischen Kinn-Referenz und unterem Figurenrand etwa 121 Einheiten, bei blecki 110;
die anatomisch verschiedenen Körperformen werden nicht umgezeichnet.

## Schläger und Zeichenreihenfolge

Die Einbuchtung war im übernommenen Pfad `blade` selbst enthalten. Kein weißer
Gesichts- oder Schmuckpfad war als nachträgliche Überdeckung nachweisbar. Dieser
Pfad ist durch einen vollständigen runden Schlägerkopf ersetzt; seine Achsen
unterschreiten 80 % der vorherigen Fläche vor der gemeinsamen Rahmung nicht.

Neue Datei `web/static/img/pip/racket.svg`: Griff, Streifen, haltende Flosse und
Schlägerfläche. `Kit` zeichnet nur bei hank **Basis → Gesicht → Schmuck → racket**.
Innerhalb von `racket` liegt die Flosse über dem Griff; die vollständige
Schlägerfläche folgt darauf und behält dadurch ihren runden Rand auch an der Hand.
Die anderen Avatare behalten ihre Zeichenreihenfolge.

- Wangenüberdeckung: **46,75 Einheiten** im geprüften Querschnitt.
- Kleinster Abstand zu Auge/Braue: **16,41 Einheiten**.
- Kleinster Abstand zum Schnabel: **23,61 Einheiten**.
- Kleinster Canvas-Rand von Schläger/Griff: **8,36 Einheiten**.
- Sichtbare gewählte Grifffarbe: **565,24 Einheiten²**, Bounding-Box ca. 31,8 × 39,2.

## Emblem und Schnabel

Das Emblem bleibt vollständig sichtbar: **48 × 48**, unterer Abstand **8,36**.
Es wird vor der Rahmung auf y=347 angehoben. Farbregel, Spielername, Großbuchstaben,
Schläger-Rückfall und Speichern bleiben unverändert; Glyphen und Rückfall werden
passend zur neuen Größe als Pfade neu erzeugt. Zum Griff/Schläger bleiben **24,67**,
zur haltenden Flosse **49,33**, zur nächstgelegenen Oberarmkontur **12,33 Einheiten**.
Die Tests berücksichtigen dabei auch die schwarzen Armkonturen.

Nur Wütend erhält einen neuen, schmaleren geschlossenen Schnabel mit nach unten
gezogenen Seiten. Die anderen fünf Schnäbel erhalten ausschließlich die gemeinsame
Rahmung; ein Test vergleicht sie geometrisch mit dem vorherigen Stand.

## Prüfbilder und Tests

- `sthings-hank-192-app.png`: alle sechs Ausdrücke nebeneinander auf Navy.
- `selection-60-app.png`: alle drei Figuren in der App-Thumbnailgröße 60 px.
- `selection-app.png`: echte Figurenauswahl aus dem Browsertest.
- `emblems-{192,64}-{app,black}.png`: A und Rückfall, Neutral und Wütend.
- `cheek-racket-comparison-420.png`: sthings, hank vorher/nachher im gleichen
  Canvas-Ausschnitt, je 420 px. Die zugrunde liegenden tatsächlich ausgelieferten
  HTTP-SVGs und die Verifikation der aktuellen Pfade liegen daneben. Der öffentliche
  Zufalls-Endpunkt hat zufällige Farben; die Farben werden nicht nachträglich ersetzt.
- `../expressions-{192,64,28}-{app,black}.png`: sechs Ausdrücke × drei Grundfarben.
- `framing-measurements.json`, `pose-measurements.json`: vollständige Messwerte.

Die Mindestunterscheidbarkeit in 64-px-GIFs beträgt **213**, schwächstes Paar
**Skeptisch/Wütend**, gegenüber zuvor 157. Go-, Browser- und Geometrietests prüfen
zusätzlich die letzte `racket`-Gruppe, vollständige Rundung, Abstände, Schopfgrenze,
Emblemgröße und die verbotenen SVG-Elemente/Attribute. Die Prüfung am echten
LED-Panel und die optische Abnahme bleiben manuell; Initialen sind bei 64 px nur etwa 3–4 Pixel hoch und damit eingeschränkt lesbar.

```sh
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python scripts/measure_hank_framing.py
go run ./scripts/avatar_review.go -framing-only -out docs/avatar-check/pip/framing
go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python scripts/measure_hank_pose.py
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python -m unittest discover -s scripts -p 'test_hank_*geometry.py' -v
# Mit lokal laufender App und Playwright:
SP_PLAYWRIGHT_MODULE=/path/to/playwright SP_PREVIEW_BROWSER_BINARY=/path/to/chrome node scripts/hank_framing.browser.cjs
```

Geänderte Implementierung: Importer, zentraler Frame-Schritt, Emblem-Generator,
`internal/avatar/render.go`, generierte hank-SVGs/Glyphen, Go-/Browser-/Geometrietests
und Prüfbild-Skripte. Keine Datenbankmigration und keine Änderung anderer Avatare.
