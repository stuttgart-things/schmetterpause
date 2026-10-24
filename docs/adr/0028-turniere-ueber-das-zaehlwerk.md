# ADR-0028: Turnierspiele kommen über das Zählwerk

- **Status:** proposed — Entwurf, nicht entschieden
- **Datum:** 2026-10-24
- **Betrifft:** Schnittstellen, Turniere
- **Bezug:** supersedes `0015-zaehlwerk-traegt-ergebnisse-ein` im Punkt „Was
  ausdrücklich wartet: Turniere über das Zählwerk“ — dort bleiben
  `tournament_id` und `tournament_round` leer, hier nicht mehr. Alles andere
  in 0015 gilt weiter, ebenso 0024 (angemeldete Matches) und 0027 (die
  Sandbox wertet sofort). Gegenstück: zaehlwerk ADR-0017. Issue #340.

## Kontext

Ein Turnierspiel hat einen Platz im Tableau: eine Runde und eine Paarung, beim
Finale die zwei, die die Tabelle nennt. Heute kommt es nur über zwei Wege
herein, den Kiosk (`handleTournamentRecord`) und das eigene Handy (ADR-0010).
Beide prüfen denselben Platz mit `checkSlot`: Das Turnier ist offen, beide
stehen im Feld, die Paarung steht in dieser Runde im Spielplan, der Platz ist
noch frei, und ein Finale ist erst dann eins, wenn die Tabelle es festlegt.

Das Zählwerk kennt kein Turnier. ADR-0015 hat die Frage, wer am Tisch den Platz
im Tableau vergibt, ausdrücklich offen gelassen. Zwei Dinge machen sie jetzt
dringend:

- **Der Turnierabend am echten Tisch.** Mit Tastern und Anmeldung zählt das
  Zählwerk das Spiel ohnehin. Wird das Ergebnis danach noch einmal am Kiosk
  getippt, ist das doppelte Arbeit und eine zweite Fehlerquelle.
- **Die Sandbox (ADR-0027).** Dort spielt Pong schon eine Liga, Einzelmatch um
  Einzelmatch. Ein ganzes Turnier bis zum Finale durchzuspielen, bevor es im
  Büro eins gibt, war der Grund, die Sandbox zu bauen.

Wer den Platz vergibt, ist damit beantwortet, sobald man die Frage umdreht:
**diese Anwendung.** Der Spielplan ist hier berechnet und nirgends gespeichert
(ADR-0011). Ein zweiter Ort, der ihn kennt, wäre eine Kopie, die auseinanderläuft.
Das Zählwerk soll also nicht wissen, welche Paarungen es gibt. Es fragt, und es
meldet das Ergebnis mit dem Platz zurück, den es genannt bekam.

## Entscheidung

**Zwei lesende Endpunkte für offene Turniere und ihre offenen Paarungen, und
`POST /api/results` nimmt einen Turnierplatz an — geprüft mit genau der Prüfung,
die Kiosk und Handy schon haben.**

### Lesen

- **`GET /api/tournaments`** listet die offenen Turniere: `id`, `name`,
  `best_of`, `points_to_win`, `side_a`, `side_b`, `rated`, die Zahl der Spiele
  insgesamt und die der gespielten. Geschlossene fehlen: Ein Zählwerk hat mit
  ihnen nichts mehr zu tun.
- **`GET /api/tournaments/{id}/pairings`** listet die **offenen** Plätze in der
  Reihenfolge des Spielplans: `round`, `home_id`, `away_id` und die zwei
  Anzeigenamen. `home` ist Seite A (ADR-0013), und das ist die linke
  Tischhälfte des Zählwerks (zaehlwerk ADR-0005). Ein Platz ist offen, solange
  kein Ergebnis darin steht, gleich in welchem Status: ein wartendes Ergebnis
  belegt ihn, wie bei `checkSlot`. Das Finale erscheint erst, wenn die Tabelle
  es festlegt, und dann als letzte Runde.
- **Berechnet, nicht gespeichert.** Die offenen Plätze sind
  `tournament.Draw` minus die belegten, plus `tournament.Final`, wenn es
  feststeht. Das wird eine reine Funktion im Paket `tournament`, und
  `checkSlot` benutzt dieselbe: Was der Endpunkt als offen anbietet, ist genau
  das, was `POST /api/results` annimmt. Zwei Rechnungen dafür wären der Anfang
  davon, dass das Zählwerk einen Platz angeboten bekommt, den es nicht füllen
  darf.
- Dasselbe Token wie der Rest von `/api` (ADR-0015). Ohne Token gibt es die
  Routen nicht.

### Schreiben

- **`POST /api/results` nimmt `tournament_id` und `round` an**, beide oder
  keins. Ohne sie bleibt alles, wie es ist.
- **Mit ihnen gilt `checkSlot`**, über dieselbe Funktion wie Kiosk und Handy:
  Turnier offen, beide im Feld, Paarung in dieser Runde, Platz frei, beim
  Finale die zwei aus der Tabelle. Eine Absage ist `409`, wenn der Platz schon
  belegt ist (das Zählwerk wiederholt dann ein Ergebnis, das angekommen ist),
  sonst `422` mit dem deutschen Satz, den `checkSlot` liefert.
- **Der Modus muss der des Turniers sein.** Anders als am Kiosk wird er nicht
  überschrieben, sondern verglichen: Weichen `best_of` oder `points_to_win`
  ab, ist das `422`. Das Zählwerk hat den Modus aus `GET /api/tournaments`
  bekommen. Ein anderer heißt, es wurde ein anderes Spiel gespielt, und das
  gehört nicht in dieses Tableau.
- **Beide Nachweise gehen**: der Operator (ADR-0015) und die zwei Anmeldungen
  (ADR-0024). Ein Turnierabend am Tisch ist genau der Fall, für den die
  Anmeldung gebaut wurde.
- **Gewertet wird wie jedes Zählwerk-Ergebnis**: `pending` oder sofort, nach
  `SP_SCOREBOARD_SETTLE` (ADR-0027), und bei einem Turnier ohne Wertung bewegt
  sich nichts (ADR-0012). `entered_via` bleibt `scoreboard` bzw. `checkin`.

### Was das Zählwerk tut

Das regelt zaehlwerk ADR-0017. Kurz: Es reicht `tournament_id` und `round` als
Teil des Handovers durch, ohne sie zu prüfen. Die Seite bietet einen Platz aus
der Liste an, und Pong spielt in der Sandbox die offenen Plätze eines Turniers
der Reihe nach, das Finale eingeschlossen.

## Warum nicht anders

**Den Spielplan dem Zählwerk geben.** Es könnte die Paarungen einmal holen und
abarbeiten. Dann wüsste es nicht, dass ein Platz inzwischen am Kiosk gefüllt
wurde, und es könnte kein Finale kennen, das erst die Tabelle festlegt. Die
Liste wird bei jedem Spiel neu gelesen, wie die Spielerliste (ADR-0015).

**Den Modus überschreiben wie am Kiosk.** Am Kiosk ist der Modus ein
Formularfeld, das jemand falsch stehen lassen kann, und das Tableau sagt, wie
gespielt wurde. Beim Zählwerk ist der Modus das, was wirklich gespielt wurde:
Die Satzlogik dort hat danach gezählt. Ihn zu überschreiben, würde ein 11:9
in einem Spiel bis 21 als gültig eintragen.

**Jede Paarung als eigenen Datensatz speichern.** Das wäre ein Spielplan in
der Datenbank, und ADR-0011 hat genau das verworfen: Er ist eine Funktion der
Spielerreihenfolge und der Ergebnisse.

**Turniere über die API anlegen.** Ein Turnier legt ein Mensch auf der
Turnierseite an, auch in der Sandbox. Ein Endpunkt dafür hätte keinen
Konsumenten, und `/api` bleibt so schmal, wie ADR-0015 es will.

## Konsequenzen

- `/api` hat danach `players`, `operators`, `results`, `tournaments` und
  `tournaments/{id}/pairings` (und den Fob-Resolve aus ADR-0025). Weiter nur
  für das Zählwerk, hinter einem Token.
- Ein Turnierabend am Tisch braucht keinen Kiosk mehr, um Ergebnisse
  einzutragen. Der Kiosk bleibt für den Abend ohne Zählwerk.
- **Ein Ergebnis kann am Platz scheitern, nachdem gespielt wurde**: wenn
  derselbe Platz in der Zwischenzeit am Kiosk gefüllt wurde. Das Zählwerk
  meldet nicht zweimal und verwirft (zaehlwerk ADR-0004). Der Fall ist
  sichtbar: Im Zählwerk-Log steht die Absage, und im Tableau steht das andere
  Ergebnis.
- **In der Sandbox** spielt Pong damit ein Turnier bis zum Finale durch, das
  ein Mensch in der Sandbox-Instanz angelegt hat. Tabelle, Finale und
  Wertung werden so geprüft, bevor das Büro einen Turnierabend über das
  Zählwerk spielt.
- Die Turniertabelle wird nicht anders: Sie zählt bestätigte Matches, auch die
  aus dem Zählwerk.

## Was ausdrücklich wartet

- **Ein Turnier über die API schließen.** Nach dem Finale bleibt es offen, bis
  es jemand auf der Turnierseite schließt, wie heute.
- **Mehrere Tische.** Zwei Tische, die gleichzeitig Plätze desselben Turniers
  spielen, brauchen keine Reservierung: Der zweite, der denselben Platz
  meldet, bekommt `409`. Einen Platz zu reservieren, bevor gespielt wird, ist
  eine eigene Frage (#280).
