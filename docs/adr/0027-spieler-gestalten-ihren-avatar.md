# ADR-0027: Spieler gestalten ihre Figur selbst

- **Status:** proposed — Entwurf, nicht entschieden
- **Datum:** 2026-10-27
- **Betrifft:** Datenmodell, Oberfläche
- **Bezug:** ersetzt die Ableitung der Schlägerfarbe aus der Spieler-ID
  (`templates.PaddleClass`, Issue #64) für Spieler, die selbst gewählt haben.
  Hebt die Grenze „Avatare pro Person“ in `docs/maskottchen-briefing.md` auf.

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

**Jeder Spieler kann seine Figur aus festen Teilen zusammenstellen. Die Wahl
wird gespeichert. Wer nicht wählt, sieht aus wie bisher.**

### Was wählbar ist

| Teil | Auswahl |
|---|---|
| Fell | Palette |
| Kopfschmuck | Palette |
| Schlägerfläche | Palette, die sieben heutigen Farben plus das Rot der Zeichnung |
| Griff | Palette |
| Gesicht | neutral, fröhlich, überrascht, skeptisch, wütend, traurig |
| Schmuck | keiner oder eins der sechs Stücke |

1. **Paletten, kein freier Farbwähler.** Jede Farbe ist eine CSS-Klasse in
   `app.css`, so wie heute `.paddle-0` bis `.paddle-6`. Dadurch lässt sich der
   Abstand zwischen Fell und Schläger vorab prüfen: Ein oranger Schläger auf
   orangem Fell ist gar nicht erst wählbar. Außerdem gelangt keine Eingabe
   eines Spielers roh in ein `style`-Attribut.
2. **Das Gesicht wählt der Spieler fest.** Die App wechselt es nicht je nach
   Ereignis. Das Briefing hat einen traurigen Ausdruck ausgeschlossen, weil
   ein Maskottchen nicht auf Niederlagen reagieren soll. Das gilt hier nicht:
   Wer „traurig“ oder „wütend“ wählt, zeigt sich selbst so, und die App
   kommentiert damit kein Ergebnis.
3. **Schmuck ist frei.** Kein Stück ist eine Auszeichnung. Die Champion-Kette
   gibt es also nicht nur für Turniersieger. Wenn das später gewünscht ist,
   braucht es eine Herleitung aus den Turnierdaten und ein eigenes ADR.

### Wo die Figur erscheint

Nur dort, wo heute die Schlägerfarbe des Spielers erscheint: in der Kopfzeile
der Startseite und auf dem Profil. Rangliste, Matchliste, Turnierseiten und
Kiosk bleiben, wie sie sind. Die Seiten mit festen Farben (`PaddleKiosk`,
`PaddleSheet`, …) zeigen weiter das Standard-Maskottchen.

Eine Figur pro Seite hält auch die Größe im Rahmen. Die Grafik wird inline
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
hängt weder an der Datenbank noch an HTTP und ist per Unit-Test abgedeckt. Es
liefert nur das gewählte Gesicht und das gewählte Schmuckstück aus, nicht alle
Varianten mit CSS zum Ausblenden. Pfade, die es nicht braucht, kommen gar nicht
erst auf die Seite.

Die Teile liegen als SVG-Fragmente unter `web/static/img/sthings/`. Sie halten
die Regeln aus `docs/maskottchen-briefing.md` ein: keine `<style>`-Blöcke, kein
`stroke`, nur gefüllte Pfade, Farben über CSS-Variablen. Die Schlägerfläche
behält `fill="var(--paddle, #c82828)"`.

### Bearbeiten

Auf dem eigenen Profil. Ein Formular mit Auswahlfeldern pro Teil. Jede Änderung
schickt HTMX an den Server, und der antwortet mit der neu zusammengesetzten
Figur als Fragment. Erst „Speichern“ schreibt in die Datenbank. Dafür braucht
es kein handgeschriebenes JavaScript (Invariante 7).

## Konsequenzen

- `PaddleClass` bleibt bestehen. Es liefert den Standard für alle, die nicht
  gewählt haben, und `avatar.Default` ruft es auf.
- Zwei Spieler können gleich aussehen. Wie heute steht der Name immer daneben,
  und der Aushang unter `/qr` zeigt ohnehin nur das Standard-Maskottchen.
- Werden zwei Spieler zusammengeführt (ADR-0017), behält der verbleibende
  seine Figur, und die Zeile des anderen fällt mit ihm weg.
- Die Prüfungen aus dem Briefing gelten für jedes neue Teil: Bei 28 px muss es
  erkennbar sein, und in Graustufen müssen sich die Gesichter unterscheiden.
- Eine neue Palettenfarbe ist eine Zeile CSS und ein Eintrag in
  `internal/avatar`. Eine Farbe herauszunehmen ist harmlos, weil der
  gespeicherte Wert auf den Standard zurückfällt.

## Verworfen

- **Freier Farbwähler.** Damit ließen sich auch unlesbare Kombinationen wählen,
  und ein Hex-Wert aus Nutzereingabe würde in ein `style`-Attribut wandern.
- **Alle Varianten in einer Datei, per CSS ausgeblendet.** So sah es das
  Briefing für Ausdrücke vor, die die App selbst wechselt. Bei einer festen
  Wahl pro Spieler wären die übrigen Varianten totes Gewicht auf jeder Seite.
- **Spalten in `players`.** Das Aussehen hat mit Wertung und Anmeldung nichts zu
  tun. Eine eigene Tabelle hält `players` schmal, und „nicht gewählt“ ist dann
  schlicht eine fehlende Zeile statt sechs `NULL`s.
- **Figur überall neben dem Namen.** Das wären viele Inline-SVGs pro Seite, und
  die Figur müsste weit unter 28 px schrumpfen. Lässt sich später mit eigenem
  ADR nachziehen.
