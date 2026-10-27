# Briefing: Gesichtsausdrücke fürs Maskottchen

Zum Weitergeben an jemanden, der zeichnen kann. Ausgangsdatei ist
`web/static/img/mascot.svg` — bitte damit anfangen, nicht neu zeichnen.

Warum es das Dokument gibt: die Ausdrücke sind der eine Teil von #64 und #102,
den kein Skript erledigen kann. Alles andere daran ist gebaut — die Grafik ist
längst SVG, die Schlägerfarbe kommt aus CSS, und `PageMascot` nimmt bereits
eine Klasse entgegen. Was fehlt, sind Formen. Die Anforderungen unten sind
deshalb keine Wunschliste, sondern das, woran ein Export sonst scheitert.

## Das Wichtigste in drei Sätzen

Eine einzige SVG-Datei, die **alle** Ausdrücke enthält — nicht eine Datei pro
Ausdruck. Jeder Ausdruck ist eine benannte Gruppe; die Anwendung blendet per
CSS eine davon ein. Farben, Maße und IDs der bestehenden Datei bleiben
unangetastet.

## Format

| | |
|---|---|
| Dateityp | `.svg`, eine Datei, UTF-8 |
| Koordinatensystem | `viewBox="0 0 420 420"` — **exakt so**, sonst passt nichts mehr |
| `width` / `height` | **weglassen**. Die Größe kommt aus CSS |
| Transform auf `<svg>` | keins. Innerhalb von Gruppen ist es erlaubt |

**Erlaubt:** `<path>`, `<g>`, `<circle>`, `<ellipse>`, Präsentationsattribute
wie `fill=` und `fill-rule=`.

**Nicht erlaubt:** Verläufe, Filter, Masken, `<image>`, echter Text
(`<text>`), externe Referenzen, eingebettete Schriften, `<style>`-Blöcke oder
`style="…"`-Attribute. Das Aussehen steuert die Anwendung, nicht die Datei.

**Konturen bitte in Flächen umwandeln** (Strich → Pfad). Die bestehende Datei
besteht ausschließlich aus gefüllten Pfaden; ein `stroke` würde bei 28 px
anders skalieren als der Rest.

## Farben — genau diese fünf, keine neuen

| Wert | Wofür |
|---|---|
| `#f89828` | Fell |
| `#b85818` | dunkles Fell: Ohreninnenseite, Schnauze, Schatten |
| `#080808` | Konturen, Pupillen, Mund |
| `#f8f8f8` | Augenweiß |
| `var(--paddle, #c82828)` | **die Schlägerfläche** |

Der letzte Punkt ist der einzige, bei dem ein Fehler etwas kaputt macht: die
Schlägerfläche muss `fill="var(--paddle, #c82828)"` behalten. Daran hängt, dass
jeder Spieler seine eigene Schlägerfarbe bekommt. Ein Zeichenprogramm schreibt
dort beim Export gern `#c82828` hinein — bitte danach von Hand zurückstellen.

## Aufbau: eine Datei, alle Ausdrücke

Die bestehenden IDs bleiben, wie sie sind:

```
#fur  #fur-dark  #ink  #eyes  #paddle-face
```

Dazu kommen die Ausdrücke als **Geschwister-Gruppen**, alle in derselben Datei,
alle mit eigener ID:

```xml
<g id="face">
  <g id="face-neutral"> … </g>
  <g id="face-happy">   … </g>
  <g id="face-wow">     … </g>
  <g id="face-focus">   … </g>
</g>
```

Jede Gruppe enthält **das komplette Gesicht** dieses Ausdrucks — Augen, Pupillen
und Mund zusammen. Nicht Augen und Mund getrennt kombinierbar machen: das
vervielfacht die Fälle und niemand prüft sie alle.

Wenn Augen und Mund heute in `#ink` und `#eyes` stecken (tun sie), müssen die
beiden Pfade so aufgeteilt werden, dass das Gesicht herauslösbar ist. Das ist
der eigentliche Handgriff und der Grund, warum das kein Skript kann.

**Alternative, falls das Zerlegen zu aufwendig ist:** die Ausdrücke als
Overlays obendrauf legen, ohne die bestehenden Pfade anzufassen. Funktioniert
auch, sieht aber schnell aufgeklebt aus — die saubere Variante ist die
Zerlegung.

## Welche Ausdrücke

Vier bis sechs. Vorschlag als Ausgangspunkt, gern anders:

| ID | Anlass in der App |
|---|---|
| `face-neutral` | Standard, Rangliste, Profil |
| `face-happy` | ein Ergebnis wurde bestätigt |
| `face-wow` | ein knappes Spiel, Verlängerung |
| `face-focus` | Kiosk-Seite, es wird gerade gespielt |

**Wichtig:** kein trauriger Ausdruck nach einer Niederlage. Die Anwendung soll
Leute zum Eintragen bringen; ein Maskottchen, das nach jedem verlorenen Spiel
enttäuscht guckt, arbeitet dagegen. Reagiert wird auf *etwas ist passiert*,
nicht auf *wer gewonnen hat*.

## Die zwei Prüfungen, an denen es scheitert

Beide bitte selbst machen, bevor abgegeben wird — sie fallen sonst erst im
Betrieb auf.

**1. Bei 28 Pixel.** So groß ist das Maskottchen in der Kopfzeile. Jeder
Ausdruck muss dort von jedem anderen unterscheidbar sein. Feine Striche
verschwinden.

**2. In Graustufen auf Weiß.** Der Aushang unter `/qr` wird gedruckt, schwarz
auf weiß. Farbe überlebt das nicht, ein Ausdruck schon — das ist überhaupt das
beste Argument für Gesichter. Also: Datei in Graustufen umwandeln, auf 40 px
verkleinern, ansehen. Wenn zwei Ausdrücke dann gleich aussehen, ist einer
davon zu subtil.

## Abgabe

- Die `.svg` ist das Ergebnis. Kein `.ai`, `.sketch` oder `.fig` als
  Lieferung — die dürfen gern dazu, aber die SVG ist das, was eingebaut wird.
- Gern mit SVGO optimiert, **aber**: die Standardeinstellung von SVGO
  **löscht IDs**. Die IDs sind hier der ganze Punkt. Also `cleanupIds`
  ausschalten (`--disable=cleanupIds`) oder unoptimiert abgeben — die Datei ist
  ohnehin nur wenige Kilobyte.
- IDs eindeutig halten. Die Grafik wird direkt in die HTML-Seite eingebettet,
  eine doppelte ID kollidiert dort mit der Seite.

## Was nicht dazugehört

Animation, ein zweites Maskottchen, alles was die Grafik aus einer externen
Quelle nachlädt.

Avatare pro Person standen hier früher auch. Seit ADR-0029 gehören sie dazu:
Jeder Spieler stellt sich seine Figur aus Fell, Kopfschmuck, Schläger, Griff,
Gesicht und Schmuck selbst zusammen. Die Format- und Farbregeln oben gelten
für diese Teile genauso. Die Grenze „kein trauriger Ausdruck“ gilt nur für das
Maskottchen, das die App selbst zeigt, nicht für eine Figur, die ein Spieler
für sich wählt.

## Die Teile unter `web/static/img/sthings/`

Aus dem Paket „Charakter und Schmuck v5“ sind Einzelteile entstanden, die die
Regeln oben einhalten: kein `<style>`, kein `stroke`, keine Transformationen,
keine Clip-Pfade, nur gefüllte Pfade mit `fill-rule="evenodd"`. Alle Teile
haben `viewBox="0 0 420 420"` und liegen ohne Verschiebung übereinander.

| Datei | Inhalt |
|---|---|
| `base.svg` | die Figur ohne Gesicht: Fell, Kopfschmuck, Griff, Schatten, Konturen, Schlägerfläche |
| `face-*.svg` | `neutral` (das bisherige Gesicht), `happy`, `surprised`, `skeptical`, `angry`, `sad` |
| `bling-*.svg` | `collar`, `scarab`, `ankh`, `horus-eye`, `chain`, `ruby` |

Gezeichnet wird in dieser Reihenfolge: `base`, dann ein Gesicht, dann
höchstens ein Schmuckstück. Wo der Schmuck hinter Gesicht oder Hand
verschwindet, ist er bereits weggeschnitten.

`base` mit `face-neutral` sieht aus wie das bisherige `mascot.svg`. Die Farben
kommen aus CSS-Variablen, die Standardwerte sind die der Zeichnung:

| Variable | Standard | Fläche |
|---|---|---|
| `--fur`, `--fur-dark` | `#f89828`, `#b85818` | Fell und seine Schatten |
| `--headdress`, `--headdress-dark` | `#f89828`, `#b85818` | farbige Streifen des Kopfschmucks |
| `--handle`, `--handle-dark` | `#f89828`, `#b85818` | Schlägergriff |
| `--paddle` | `#c82828` | Schlägerfläche, wie bisher |

Konturen (`#080808`), Augenweiß (`#f8f8f8`) und die Materialfarben des Schmucks
sind fest. Wer ein neues Teil zeichnet, prüft es so wie oben beschrieben bei
28 px und in Graustufen. Bei 28 px sind `neutral`, `skeptical`, `angry` und
`sad` heute schwer auseinanderzuhalten. Das stört nicht, solange jeder Spieler
sein Gesicht selbst wählt und es nicht als Signal dient.
