# hank – Schläger in sthings-Proportion, sichtbare Hand

## Referenz und Ergebnis

| Messwert | sthings Neutral | hank Neutral |
|---|---:|---:|
| Gerenderte Fläche bei 420 px | 130 × 135 px | 118 × 123 px |
| Vektorfläche | 130 × 135 | 118,3 × 122,85 |
| Anteil Canvas (Vektor, B/H) | 30,95 % / 32,14 % | 28,17 % / 29,25 % |
| Flächenmittelpunkt (BBox) | (348; 228,5) | (346,1; 269,8) |
| Mittlere Augenlinie | y=181,5 | y=222,25 |
| Mittelpunkt unter Augenlinie | 47,0 | 47,56 |
| Seitlicher Versatz zur Gesichtsmitte | +142,0 | +150,02 |
| Rekonstruierte Gesichtsüberdeckung | 22,35 % | 20,27 % |
| Kleinster Augenabstand zur Kontur, Neutral | ca. 3,06 | 6,24 |
| Sichtbare farbige Pfote/Flosse, 420er Raster | 6.516 Pixel; 82 × 105 px | 2.152 Pixel; 56 × 66 px |
| Sichtbarer Griff inkl. Schattierung, Vektorfläche | 503,14 | 861,92 |

Die PNG-Masken werden im gleichen Go-Rasterisierer wie die App/GIF-Vorschau bei
420 px gerendert und bei 50 % Deckung vermessen. Die Vektormaße ergänzen die
Pixelmessung. Der vollständige sthings-Flächenpfad wird für hank mit **91 %**
übernommen. Damit halten auch die gerundeten Pixelmaße die ±10-%-Vorgabe ein.
Die Mittelpunkthöhe relativ zur Augenlinie weicht um etwa 1,2 % ab. Die
Gesichtsmittelpunkte sind (206; 195,3) und (196,08; 219,09); die Augenmittelpunkte
liegen horizontal bei 203,75 und 197,22. Alle vollständigen Werte liegen in
`measurements.json` und `raster-measurements.json`.

**Überdeckung ist eine Rekonstruktion:** Bei sthings ist die sichtbare orange
Gesichtsfläche bereits am Schläger ausgespart. Ein direktes Schnittflächenmaß
wäre deshalb null. Für den verdeckten Teil wird die linke Gesichtshälfte an x=206
symmetrisch ergänzt. hank besitzt dagegen eine vollständige weiße Gesichtsfläche.
Mit dieser dokumentierten Definition unterscheidet sich der Anteil um etwa
2,1 Prozentpunkte bzw. 9,3 % relativ. Die sthings-Kontur wird mit ihrem 7-Einheiten-
Außenrand gemessen (blaue/rechte schwarze Grenze bei 413/420). Deshalb gilt für
hank der strengere Mindestwert **6**, nicht der kleinere Referenzabstand.

Die Griff-Boundingbox bei sthings umfasst mehrere getrennte sichtbare Teile;
sie bedeutet keine durchgehend gefüllte Griff-Rechteckfläche. Bei der Flosse
werden farbige Oberfläche und volle schwarze Kontur getrennt gemessen. Ihre
Anatomie ist nicht auf dieselbe Größe wie die Katzenpfote skaliert.

## Rahmung und Haltung

Gewählt sind **110 %**: Schon die kleinste erlaubte Skalierung erfüllt alle
Haltungs- und Emblemregeln. Kopf und Stirnband werden gleichmäßig skaliert; der
Schopf behält seine 40-%-Kürzung und 8 Einheiten oberen Abstand. Die Figur liegt
etwas weiter links, damit der größere Schläger ohne Augenüberdeckung und mit
8 Einheiten Canvas-Rand Platz hat. Die Kopfbreite ist gegenüber dem nativen
Stand vor 5c43775 weiterhin um 10 % größer.

Die Körperdarstellung reicht nun bis ursprünglich y≈405 statt y≈374 beim
120-%-Ausschnitt. Unter dem Kinn bleiben etwa **99 statt 71 Einheiten** sichtbar.
Der Oberarm der Schlägerseite ist mit der vollständig sichtbaren Flosse verbunden.
Alle sechs Gesichter einschließlich sämtlicher Schnäbel werden ausschließlich
mit der gemeinsamen Rahmung verändert. Andere Avatare und Paletten bleiben gleich.

Die eigene letzte `racket`-Gruppe bleibt erhalten. Der vollständige sthings-Pfad
liegt vor Gesicht und Schmuck; die Flosse liegt über dem Griff und unter der
Schlägerfläche. Die Flosse ist geometrisch zu **100 % sichtbar**, einschließlich
Spitze und schwarzer Außenkontur, und wird nicht am Canvas abgeschnitten.

- Kleinster Augen-/Brauenabstand über alle sechs Ausdrücke: **6,23 Einheiten**.
- Kleinster Schnabelabstand: **26,33 Einheiten** (gefordert mindestens 8).
- Griff: **861,92 Einheiten²** sichtbare gewählte Farbe, über dem Ziel 30 × 18.
- Schläger-/Griff-Canvasrand: mindestens **8,25 Einheiten**.
- Die gesamte Flossenkontur bleibt ebenfalls mindestens 8 Einheiten im Canvas.

## Emblem

Das Emblem bleibt **48 × 48**. Es hat etwa **29,08 Einheiten zum nächsten Schnabel**
und **24,60 zum unteren Rand**. Zu Griff/Schläger bleiben mindestens **39,15**,
zur haltenden Flosse **62,15** Einheiten. Auch die Oberarmkonturen werden mit
mindestens 12 Einheiten geprüft. Es sitzt damit frei auf der Brust und wirkt im
Prüfblatt nicht an den Schnabel gedrängt. Die endgültige optische Abnahme liegt
beim Nutzer. Farbregel, Spielername, Großbuchstaben und Rückfall-Symbol bleiben.

## Prüfbilder und Tests

- `sthings-hank-192-app.png`: sechs Ausdrücke ohne Schmuck auf Navy.
- `sthings-hank-emblem-192-app.png`: dasselbe Blatt mit hanks Initiale A.
- `selection-60-app.png`: sthings, blecki und hank in App-Kachelgröße.
- `selection-app.png`: echte Figurenauswahl aus dem Browsertest.
- `../expressions-{192,64,28}-{app,black}.png`: sechs Ausdrücke × drei Grundfarben.
- `*-mask.svg` / `*-mask.png`: sichtbare Fläche, Pfote/Flosse und Griff, ohne
  Übermalung der Originaldateien und mit reproduzierbarer Pixelmessung.

Die Geometrietests vergleichen Größe und Position relativ zur unveränderten
sthings-Referenz, die rekonstruierte Überdeckung, die Mindestabstände, vollständige
Flossensichtbarkeit, Griff-Fläche, letzte Zeichengruppe und Emblemregeln. Ein neuer
Go-Test prüft die ±10 % auch am tatsächlichen 420er-Raster. Gesichtsinvarianz,
SVG-Verbote sowie Speichern, Würfeln, Zurücksetzen und SVG/GIF bleiben geprüft.

Die Mindestunterscheidbarkeit in 64-px-GIFs ist **193**, schwächstes Paar
**Skeptisch/Wütend**, über der geforderten Grenze 157. Offen bleiben die optische
Abnahme und die Lesbarkeit am echten LED-Panel, insbesondere der kleinen Initiale.

```sh
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python scripts/measure_hank_holding.py
go run ./scripts/avatar_review.go -holding-only -out docs/avatar-check/pip/holding
go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python scripts/measure_hank_pose.py
PYTHONDONTWRITEBYTECODE=1 /tmp/avatar-vector-tools/bin/python -m unittest discover -s scripts -p 'test_hank_*geometry.py' -v
go test ./...
```

Geänderte Implementierung: `hank_frame.py`, `hank_pose.py`, Importer und gebackene
hank-SVGs/Glyphen; Referenz- und Messskripte; Go-/Geometrietests und Prüfbild-Skript.
Keine Datenbankmigration oder Änderung der Slot-/Farb-Logik.
