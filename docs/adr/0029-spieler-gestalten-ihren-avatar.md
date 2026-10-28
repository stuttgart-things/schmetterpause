# ADR-0029: Spieler gestalten ihre Figur selbst

- **Status:** accepted
- **Datum:** 2026-10-27
- **Betrifft:** Datenmodell, Oberfläche
- **Bezug:** ersetzt die Ableitung der Schlägerfarbe aus der Spieler-ID
  (`templates.PaddleClass`, Issue #64) für Spieler, die selbst gewählt haben.
  Hebt die Grenze „Avatare pro Person“ in `docs/maskottchen-briefing.md` auf.
  Betrifft auch stuttgart-things/zaehlwerk, das die Schlägerfarbe heute mit
  derselben Formel nachrechnet (`internal/ui/render.go`, `blade`). Die
  Zählwerk-Seite, Gesichter im Stream und die Figuren auf dessen Bildschirm,
  ist stuttgart-things/zaehlwerk#195.

## Kontext

Das Maskottchen sthings steht neben der Überschrift. In der Kopfzeile und auf
dem Profil trägt es die Schlägerfarbe des Spielers. Diese Farbe wird aus der
ID berechnet (FNV-1a über sieben Farben), nicht gespeichert. Das war bewusst
so entschieden: kein Feld, keine Auswahl, keine Migration. Der Preis steht im
Kommentar daneben: niemand kann seine Farbe wählen, und ab sieben Spielern
teilen sich zwei eine.

Inzwischen gibt es Material für mehr als eine Farbe. Das Paket
„Charakter und Schmuck v5“ teilt das Fell in Körper, Kopfschmuck und Griff auf.
Es bringt fünf Gesichtsausdrücke und sechs Schmuckstücke mit, alle auf
derselben `viewBox="0 0 420 420"` wie `web/static/img/mascot.svg`. Damit kann
sich jeder Spieler eine eigene Figur zusammenstellen. Genau das soll er auch
können: Eine Figur, die man selbst gebaut hat, macht die eigene Seite zur
eigenen.

## Entscheidung

**Jeder Spieler kann seine Figur aus festen Teilen zusammenstellen und sie
jederzeit wieder ändern. Die Wahl wird gespeichert. Wer nicht wählt, sieht aus
wie bisher. Demo-Spieler bekommen eine Figur aus dem Zufall. Das Gesicht ist
keine Wahl: Es gehört zum Spiel, und das Zählwerk bestimmt es aus dem
Spielstand.**

### Was wählbar ist

| Teil | Auswahl |
|---|---|
| Fell | Orange, Ingwer, Zimt, Gold, Sand, Creme, Weiß, Schoko |
| Kopfschmuck | wie Fell, Gold, Lapis, Türkis, Smaragd, Rubin, Elfenbein, Silber |
| Schlägerfläche | die sieben heutigen Farben, in derselben Reihenfolge |
| Griff | wie Fell, Holz hell, Holz dunkel, Rot, Blau, Grün, Elfenbein |
| Schmuck | keiner oder eins der sechs Stücke |

Das sind 21.952 mögliche Figuren. Die Farbwerte stehen nur in
`internal/avatar`. Ein Test dort hält sie an die folgenden Abstände. Gemessen
wird wie bei den Schlägerfarben mit dE76:

- **Fell** hält mindestens 38 zu jeder Schlägerfarbe, damit der Schläger nicht
  im Fell verschwindet, und mindestens 56 zum Grund der Seite. Grau fällt
  heraus, es liegt 11 neben „Stahl“. Der Schatten wird aus der Fellfarbe mit
  demselben Schritt abgeleitet, den die Zeichnung von Orange zu Braun geht.
- **Kopfschmuck** hält mindestens 74 zur Kontur, weil sich farbige und
  schwarze Streifen abwechseln. **Griff** hält mindestens 58 zur Kontur.
- **Schlägerfläche** bleibt bei den sieben Farben von heute. Das Rot der
  Zeichnung ist nicht wählbar: Es liegt 26 neben der Farbe für Verlust, und die
  Schlägerfarbe hält 37 Abstand zu allem, was in der App etwas bedeutet. Rot
  bleibt das Zeichen für „niemand erkannt“.

1. **Paletten, kein freier Farbwähler.** Dadurch lässt sich der Abstand
   zwischen Fell und Schläger vorab prüfen: Ein oranger Schläger auf orangem
   Fell ist gar nicht erst wählbar. Die Farben stehen als CSS-Variablen in
   einem `style`-Attribut auf dem `<svg>` der Figur, nicht als Klassen in
   `app.css`. Nur so trägt die Figur ihre Farben auch dorthin mit, wo kein
   Stylesheet von hier gilt, also ins Zählwerk. In das Attribut gelangen nur
   Werte aus den Paletten, nie eine Eingabe eines Spielers. Unbekannte Schlüssel
   fallen vorher auf den Standard zurück.
2. **Das Gesicht gehört zum Spiel, nicht zur Figur.** Ein fest gewähltes
   „wütend“ sagt nach drei Wochen nichts mehr. Ein Gesicht, das beim 9:10
   überrascht schaut, erzählt etwas über das Spiel. Bei 28 px in der Kopfzeile
   sind neutral, skeptisch, wütend und traurig ohnehin kaum zu unterscheiden.
   Am Spielstand ist die Figur groß. Abseits eines laufenden Matches zeigt sie
   immer das neutrale Gesicht. Siehe „Gesichter im Spiel“.
3. **Schmuck ist frei.** Kein Stück ist eine Auszeichnung. Die Champion-Kette
   gibt es also nicht nur für Turniersieger. Wenn das später gewünscht ist,
   braucht es eine Herleitung aus den Turnierdaten und ein eigenes ADR.

### Ändern

Der Editor auf dem eigenen Profil ist jederzeit erreichbar. Eine Änderung
überschreibt die Zeile in `player_avatars`, eine Historie gibt es nicht. Ein
Knopf „Zurücksetzen“ löscht die Zeile, und die Figur fällt auf den Standard aus
der ID zurück. Ändern kann jeder nur seine eigene Figur. ADR-0008 regelt, wer
für andere handeln darf, und Aussehen gehört nicht dazu.

### Zufall

`avatar.Random(rng)` würfelt aus jeder Palette und jedem Schmuckstück
gleichverteilt. Genutzt wird es an drei Stellen:

1. **Im Editor** gibt es einen Knopf „Würfeln“. Er füllt das Formular mit
   einer Zufallsfigur, gespeichert wird erst mit „Speichern“.
2. **Das Demo-Feld** (`schmetterpause seed`, Preview-Umgebungen) legt für jeden
   Spieler eine Zufallsfigur an. Der Generator ist mit dem Seed des Feldes
   initialisiert, damit zwei Läufe dieselben Figuren ergeben und Screenshots
   vergleichbar bleiben.
3. **Für das Zählwerk ohne Spieler**, also für Demo-Matches wie `Pong A`
   gegen `Pong B` (zaehlwerk ADR-0008). Siehe unten.
4. **`schmetterpause dress`** gibt jedem Spieler ohne Figur nachträglich eine
   Zufallsfigur, für eine Sandbox, die vor den Figuren befüllt wurde oder in
   die später jemand Spieler eingetragen hat. Eine gewählte Figur bleibt,
   Beobachter bleiben beim Standard. Der Generator ist mit der Spieler-ID
   initialisiert, damit jeder Lauf demselben Spieler dieselbe Figur gibt; ein
   zweiter Lauf ändert nichts.

### Zählwerk

Das Zählwerk zeigt beim Spielstand heute das Maskottchen mit einer
Schlägerfarbe. Die rechnet es aus der Spieler-ID nach, mit derselben
FNV-Formel wie `PaddleClass`. Sobald Spieler selbst wählen, wäre diese
Nachrechnung falsch. Das Zählwerk soll stattdessen die Figur zeigen, die der
Spieler hier gebaut hat, sofern die Spieler aus Schmetterpause kommen.

Dafür bekommt Schmetterpause zwei öffentliche Routen. Die Figur eines
Spielers ist auf seinem Profil ohnehin für jeden zu sehen, ein Token schützt
hier also nichts:

- `GET /avatars/{id}.svg` liefert die fertig zusammengesetzte Figur eines
  Spielers mit allen sechs Gesichtern. Welches zu sehen ist, entscheidet
  `data-face` auf dem `<svg>`-Element. Eine kleine `<style>`-Regel in der Datei
  blendet die anderen aus. Die Farben stehen als CSS-Variablen auf demselben
  Element, also braucht das Zählwerk kein eigenes CSS und keine eigenen Teile.
  Es lädt die Figur einmal pro Match, bettet sie inline ein und wechselt danach
  nur noch das Attribut.
- `GET /avatars/random.svg` liefert eine Zufallsfigur für Demo-Matches. Sie
  ändert sich bei jedem Aufruf, so wie heute die zufällige Schlägerfarbe im
  Zählwerk.

`GET /api/players` bekommt zusätzlich das Feld `paddle` mit der Schlägerfarbe
als Hex-Wert. Damit kann das Zählwerk seine Schiedsrichterschnur färben, ohne
die Figur zu laden.

Die Teile selbst gehen nicht ins Zählwerk. Eine einzige Quelle für die Figur
heißt, dass eine neue Palettenfarbe nur hier eingetragen wird. Ohne
`SCHMETTERPAUSE_URL` oder wenn die Route nicht antwortet, bleibt das Zählwerk
bei seinem eigenen Maskottchen. Was es dort umbaut, entscheidet ein eigenes ADR
im Zählwerk-Repository.

### Gesichter im Spiel

Das Zählwerk kennt den Spielstand und entscheidet deshalb, welches Gesicht
welche Seite gerade zeigt. Es schickt die Entscheidung in seinem Live-Stream mit,
als `faces` neben `home_id` und `away_id`. Wer den Stream anzeigt, setzt nur
noch `data-face`, ob auf dem Zählwerk-Bildschirm oder auf der Startseite hier.
Die Zuordnung gibt es so nur an einer Stelle, und `live.js` bleibt dabei, nichts
selbst zu berechnen. Im Zählwerk gehört sie in die Darstellung, nicht in den
Scorer, der dort bewusst nur die Regeln kennt.

Die endgültige Zuordnung legt das ADR im Zählwerk fest. Als Vorschlag:

| Moment | Gesicht |
|---|---|
| Einspielen, Seitenwechsel | neutral |
| Punkt gewonnen, kurz | fröhlich |
| Punkt verloren, kurz | skeptisch |
| Satzball, Matchball, Verlängerung | überrascht |
| drei verlorene Punkte in Folge | wütend |
| Satz verloren, kurz | traurig |
| Match gewonnen | fröhlich |
| Match verloren | skeptisch |

Das Briefing schließt einen traurigen Ausdruck nach einer Niederlage aus, weil
ein Maskottchen, das nach jedem verlorenen Spiel enttäuscht guckt, dagegen
arbeitet, dass Leute Ergebnisse eintragen. Am Tisch ist das anders: Beide stehen
da, das Spiel läuft, und Gefühl gehört dazu. „Traurig“ bleibt trotzdem ein kurzer
Moment nach einem verlorenen Satz. Was nach dem Match stehen bleibt, ist
„skeptisch“.

### Wo die Figur erscheint

In Schmetterpause nur dort, wo heute die Schlägerfarbe des Spielers erscheint:
in der Kopfzeile der Startseite und auf dem Profil. Dazu kommen die beiden
Spieler eines laufenden Matches: auf dem Zählwerk-Bildschirm und auf der
Startseite neben dem Spielstand, den `live.js` dort schon aus dem Stream des
Zählwerks zeigt (#188). Rangliste, Matchliste, Turnierseiten und
Kiosk bleiben, wie sie sind. Die Seiten mit festen Farben (`PaddleKiosk`,
`PaddleSheet`, …) zeigen weiter das Standard-Maskottchen.

Wenige Figuren pro Seite halten auch die Größe im Rahmen. Die Grafik wird inline
eingebettet (`web.Mascot`, Issue #64). Zwanzig davon in einer Rangliste wären
zwanzigmal die Pfade.

### Speichern

Eine eigene Tabelle `player_avatars` mit `player_id` als Primär- und
Fremdschlüssel, eine Spalte pro Teil als `text`, dazu `updated_at`. Die Migration
ist additiv (Invariante 8). Zugriff gibt es nur über ein `AvatarRepository`
(Invariante 5).

Gültig ist ein Wert, wenn das Package `internal/avatar` ihn kennt. Die Tabelle
hat keine `CHECK`-Liste. Sonst müsste für jede neue Palettenfarbe eine Migration
mitlaufen, und eine Farbe herauszunehmen wäre eine destruktive Änderung. Ein
gespeicherter Wert, den der Code nicht mehr kennt, wird wie „nicht gewählt“
behandelt und fällt auf den Standard zurück.

Fehlt die Zeile, berechnet `avatar.Default(playerID)` die Figur. Das ist die
heutige FNV-Ableitung für den Schläger, alles andere bleibt wie gezeichnet.
Nach dem Deploy sieht deshalb niemand anders aus als vorher.

### Zusammensetzen

`internal/avatar` setzt die Figur serverseitig aus Teilen zusammen. Das Package
hängt weder an der Datenbank noch an HTTP und ist per Unit-Test abgedeckt.
`Render` liefert die Figur mit dem neutralen Gesicht und nur dem gewählten
Schmuckstück, für Kopfzeile und Profil. `RenderLive` liefert alle sechs
Gesichter dazu, für den Platz neben einem laufenden Match.

Die Teile liegen als SVG-Fragmente unter `web/static/img/sthings/`. Sie halten
die Regeln aus `docs/maskottchen-briefing.md` ein: keine `<style>`-Blöcke, kein
`stroke`, nur gefüllte Pfade, Farben über CSS-Variablen. Die Schlägerfläche
behält `fill="var(--paddle, #c82828)"`.

### Editor

Auf dem eigenen Profil. Ein Formular mit Auswahlfeldern pro Teil. Jede Änderung
schickt HTMX an den Server, und der antwortet mit der neu zusammengesetzten
Figur als Fragment. „Würfeln“ ist derselbe Weg mit einer Zufallsfigur. Erst
„Speichern“ schreibt in die Datenbank. Dafür braucht es kein handgeschriebenes
JavaScript (Invariante 7).

## Konsequenzen

- `PaddleClass` bleibt bestehen. `avatar.Default` rechnet dieselbe Formel, und
  ein Test hält die beiden aneinander.
- Zwei Spieler können gleich aussehen. Wie heute steht der Name immer daneben,
  und der Aushang unter `/qr` zeigt ohnehin nur das Standard-Maskottchen.
- Werden zwei Spieler zusammengeführt (ADR-0017), behält der verbleibende
  seine Figur, und die Zeile des anderen fällt mit ihm weg.
- Die Schlägerfarbe wird zum ersten Mal zu einer Angabe, die ein anderer
  Dienst lesen muss, statt sie nachzurechnen. Bis das Zählwerk umgestellt ist,
  zeigt es für Spieler mit eigener Wahl die alte, berechnete Farbe.
- Die Prüfungen aus dem Briefing gelten für jedes neue Teil: Bei 28 px muss es
  erkennbar sein, und in Graustufen müssen sich die Gesichter unterscheiden.
- Eine neue Palettenfarbe ist ein Eintrag in `internal/avatar`. Der Test
  dort misst sie gegen die Regeln oben. Eine Farbe herauszunehmen ist
  harmlos, weil der gespeicherte Wert auf den Standard zurückfällt.

## Verworfen

- **Freier Farbwähler.** Damit ließen sich auch unlesbare Kombinationen wählen,
  und ein Hex-Wert aus Nutzereingabe würde in ein `style`-Attribut wandern.
- **Ein Gesicht, das der Spieler fest wählt.** So stand es im ersten Entwurf
  dieses ADR. Ein fester Ausdruck sagt nach kurzer Zeit nichts mehr, und bei
  28 px ließen sich vier der sechs kaum unterscheiden.
- **Die Zuordnung Spielstand → Gesicht in jedem Anzeiger.** Dann müssten
  Zählwerk und `live.js` dieselbe Tabelle pflegen, in zwei Sprachen. Sie liegt
  deshalb im Zählwerk und reist im Stream mit.
- **Spalten in `players`.** Das Aussehen hat mit Wertung und Anmeldung nichts zu
  tun. Eine eigene Tabelle hält `players` schmal, und „nicht gewählt“ ist dann
  schlicht eine fehlende Zeile statt fünf `NULL`s.
- **Figur überall neben dem Namen.** Das wären viele Inline-SVGs pro Seite, und
  die Figur müsste weit unter 28 px schrumpfen. Lässt sich später mit eigenem
  ADR nachziehen.
