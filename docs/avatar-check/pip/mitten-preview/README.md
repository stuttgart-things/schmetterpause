# hank – Freigegebener Fäustling nach sthings-Vorbild

**Freigegeben und bytegleich in den Produktstand übernommen.**
Aktueller App-/Pixelvergleich: [Produkt-Fäustling](../mitten-product/index.html).

## sthings als Referenz

Quer zur Schlägerachse, durch den Mittelpunkt des ersten geschlossenen
Faustabschnitts, misst die frontale Faust einschließlich Kontur
**96.8** Einheiten. Die Hüllbreite der sichtbaren
oberen Stielflächen beträgt **61.1**:
Verhältnis **1.58**. Die Handflächenbox insgesamt ist
(294,313)–(376,418), einschließlich Handgelenk; sie ist nicht mit der frontalen
Faustbreite entlang der gedrehten Messachse gleichzusetzen.

Am Stielzentrum sind oben **13.4**,
unten **5.7** Einheiten Stielfarbe sichtbar.
Die Vorgaben für hank verlangen bewusst mehr: mindestens 20 und ungefähr 10.

Das native sthings-SVG enthält nur einzelne sichtbare Stielfarbflächen; ein
vollständiger durchgehender Stiel hinter der Faust ist dort nicht gezeichnet.
Die später gezeichnete Tintenkontur und die runde Faustfläche fassen den Stiel
optisch von vorn ein. Der hank-Entwurf verwendet dafür einen tatsächlich
durchgehenden Stielpfad hinter der Faust.

## Entwurf

Geschlossene runde Vorderform in Flossenfarbe, Kontur 7 Einheiten und eine einzige
geschwungene Falz. Keine Finger und kein abstehender Daumen. Der breite Arm
verbindet Schulter und Faust; seine gemessenen Farbquerschnitte liegen zwischen
43.2 und
47.0 Einheiten.
Die Faustkontur ist am Handgelenk mit dem Arm verbunden, ohne einen schwarzen
Ring oder eine abgesetzte dünne Handwurzel. Der Stiel wird zuerst gezeichnet,
danach die Faust, zuletzt die unveränderte Schlägerfläche.

| Messung | Entwurf |
|---|---:|
| Faust-/Stielbreite einschließlich Kontur | 58.0 / 36.0 = **1.61** |
| Verhältnis der Farbflächen | 44.0 / 26.0 = 1.69 |
| Freier Hals oberhalb der Faust | **20.9** |
| Stielstück unterhalb der Faust bis zur Kappe | **11.1** |
| Stielachse | **13.42°** |
| Konstruktionsabweichung oberer/unterer Abschnitt | 0° – derselbe durchgehende Stielpfad |
| Emblemabstand | 17.027 ≥ 12 |
| Canvas-Abstand | 13.265 ≥ 8 |
| Gemessene Unterscheidbarkeit bei 64 px | 184 ≥ 157 |

Der untere Stielabschluss ist rund und hat eine farblich abgesetzte Kappe.
Schlägerfläche, Gesichter, Emblem, linke Flosse und Rahmung bleiben a99eac2.
Nur die beiden rechten Armpfade der isolierten Basis und der isolierte
Stiel-/Faustaufbau wurden für die Ansicht geändert.

## Prüfbilder und Rendering

- `hand-comparison-420.png`: **sthings | hank a99eac2 | hank Fäustling**, je 420 px.
  Identischer Ausschnitt, roter Stiel zur Sichtkontrolle.
- `expressions-192-navy.png`: alle sechs Ausdrücke, je 192 px.
- `expression-{neutral,happy,surprised,skeptical,angry,sad}-192.png`: Einzelbilder.
- `live-expressions.html`: dieselben sechs echten Produktions-`RenderLive`-SVGs
  mit der unveränderten App-CSS-Datei. Chromium hat je genau einen aktiven
  Gesichtszustand geprüft und daraus die 192-px-PNGs aufgenommen.
- `mitten-base.svg`, `mitten-racket.svg`: ausschließlich isolierte Entwurfsassets.
- `measurements.json`, `renderer-verification.json`, `browser-verification.json`:
  Messungen und Renderer-Nachweise.

Die Ausdrucksbilder verwenden **den echten App-Renderer samt App-CSS**, mit
Entwurfsassets ausschließlich in einer privaten In-Memory-Collection. Die
laufende App auf Port 8080 wird für diese Freigabeansicht nicht verändert.
Die Produktübernahme ist freigegeben; aktuelle Tests und Nachweise stehen im Produktbericht.

Reproduktion aus dem Repository mit Vector-Tools-Python:
`python docs/avatar-check/pip/mitten-preview/generate.py`, danach
`go run ./docs/avatar-check/pip/mitten-preview/render.go`,
`node docs/avatar-check/pip/mitten-preview/capture.cjs` mit verfügbarem Playwright,
und `python docs/avatar-check/pip/mitten-preview/report.py`.
Bild-Cachekennung: `785d3ab6699b897b`.
