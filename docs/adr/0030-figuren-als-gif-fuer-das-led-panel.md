# ADR-0030: Die Figuren gibt es auch als GIF, für das LED-Panel

- **Status:** accepted
- **Datum:** 2026-10-28
- **Betrifft:** Oberfläche, Abhängigkeiten
- **Bezug:** ergänzt ADR-0029 um ein zweites Format derselben Figur. Issue
  #366. Die Gegenstücke sind stuttgart-things/zaehlwerk#204 (schickt die
  Adresse des Siegers ans Panel) und
  stuttgart-things/homerun2-led-catcher#148 (spielt ein Bild, das die
  Nachricht wählt).

## Kontext

Am Tisch hängt ein LED-Panel mit 64 × 64 Pixeln. Der homerun2 led-catcher
treibt es und zeigt dort während eines Matches den Spielstand. Bilder und GIFs
kann er auch abspielen, inzwischen sogar von einer `https://`-Adresse. SVG kann
er nicht.

Unsere Figuren sind SVG, und zwar solche, die einen Browser voraussetzen. Die
Farben stehen als Custom Properties am Wurzelelement, die Teile lesen sie mit
`var(--fur, …)`. Welches Gesicht zu sehen ist, entscheidet eine `<style>`-Regel
über `data-face`. Ein Rasterer ohne CSS-Engine zeichnet daraus eine farblose
Figur mit sechs Gesichtern übereinander.

Ein Versuch hat zwei Fragen geklärt:

- **Die Figur ist auf 64 × 64 noch zu erkennen**, mit Gesicht, Fell,
  Kopfschmuck, Schmuck und Schläger. Auf 32 × 32 bleiben nur die Farben, das
  Gesicht geht verloren.
- **Für das Rastern braucht es keinen Browser.** Setzt man vorher die Farben
  ein und behält nur das eine Gesicht, zeichnet `github.com/srwiley/oksvg`
  die Figur in reinem Go so, wie Chrome sie zeichnet.

## Entscheidung

**schmetterpause liefert jede Figur auch als GIF aus, unter derselben Adresse
wie das SVG, nur mit `.gif`. Gezeichnet wird sie in reinem Go mit `oksvg` und
`rasterx`, aus einer flachen Fassung der Figur.**

- **Die Routen:** `GET /avatars/{id}.gif` und `GET /avatars/random.gif`. Sie
  sind öffentlich und werden so zwischengespeichert wie das SVG daneben: eine
  Minute für einen Spieler, gar nicht für eine Zufallsfigur.
- **Ohne `?face=`** jubelt die Figur. Das ist eine Schleife aus fünf Bildern:
  zweimal erfreut und kurz verblüfft, dann ein Atemzug neutral
  (`avatar.Cheer`). Sie ist für den Moment nach einem gewonnenen Satz oder
  Match gemacht.
- **Mit `?face=`** gibt es ein Standbild mit diesem Gesicht. Ein unbekanntes
  Gesicht wird zum neutralen, wie beim SVG.
- **`?size=`** ist die Seitenlänge in Pixeln. Ohne Angabe sind es 64, und die
  Größe bleibt zwischen 32 und 256. Darunter ist das Gesicht weg, darüber
  braucht es kein Panel mehr, und eine öffentliche Route soll nicht jede Größe
  zeichnen, die jemand bestellt.
- **Die flache Fassung** (`Kit.RenderFlat`) hat die Farben eingesetzt, genau
  ein Gesicht und kein CSS. Sie entsteht aus denselben Teilen wie das SVG, die
  Figur gibt es also nur einmal.
- **Die Palette** besteht aus Schwarz, der Farbe eines ausgeschalteten
  Panels, und den Farben der Figur, sonst nichts. Jeder Pixel nimmt die
  nächstliegende davon, ohne Dithering. Die geglätteten Kanten würden sonst zu
  einem Gesprenkel aus Zwischenfarben, und auf einem LED-Raster liest sich das
  als Rauschen statt als Linie.

## Konsequenzen

- **Neue Abhängigkeiten:** `oksvg` und `rasterx`, dazu `golang.org/x/image`.
  Beide sind reines Go, ohne cgo und ohne Browser im Image (Invariante 1).
  Beide sind seit 2022 nicht mehr verändert worden. Für das, was sie hier
  zeichnen, reicht das: Pfade mit Füllung und `fill-rule`, mehr verwenden die
  Teile nicht. Dafür sorgt `TestPartsFollowTheBriefing`, der Strich,
  Transformation, Verlauf und Filter in den Teilen verbietet. Kommt doch etwas
  davon, muss jemand diesen Test ändern und stößt dabei auf diese
  Entscheidung.
- **Rechenzeit pro Anfrage:** Gemessen braucht die Jubel-Schleife in 64
  Pixeln rund 13 ms und in 256 Pixeln rund 100 ms. Das Panel fragt einmal pro
  gewonnenem Satz. Zwischengespeichert wird im Server nichts. Wird die Route
  zum Lastproblem, kommt als Nächstes ein Cache pro Figur und Größe, oder eine
  kleinere Obergrenze.
- **Das SVG bleibt das Format für Seiten.** Das GIF ist nur für Geräte
  gedacht, die kein SVG können.
