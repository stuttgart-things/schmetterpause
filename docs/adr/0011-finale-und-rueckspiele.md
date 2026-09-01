# ADR-0011: Ein Finale ist eine abgeleitete Paarung, kein gespeicherter Spielplan

- **Status:** accepted
- **Datum:** 2026-09-01
- **Betrifft:** Turniere, Datenmodell
- **Bezug:** schreibt `0009-schnelles-turnier-wertet-pro-match` fort und
  beantwortet dessen Kernsatz für einen Fall, den es nicht kannte; ersetzt kein
  ADR. Setzt Issue #132 um.

## Kontext

Zwei Wünsche aus dem Büro sehen aus wie zwei Features und sind einer:
**Rückspiele** und ein **Finale**. Beide scheitern heute an derselben Stelle.

Der Spielplan ordnet Ergebnisse über das Paar zu:

```go
played[pairKey(m.HomeID, m.AwayID)] = m
```

`pairKey` ist absichtlich symmetrisch. Für ein einfaches Round Robin ist das
exakt richtig — jede Paarung kommt genau einmal vor —, und für alles andere
falsch. Ein Rückspiel A–B fände das Ergebnis des Hinspiels unter seinem
eigenen Schlüssel, zeigte es an und bekäme nie ein Eingabefeld. Ein Finale
zwischen zwei Leuten, die in der Gruppe schon gegeneinander gespielt haben,
kollidiert genauso.

Die Tabelle ist davon nicht betroffen: `tournament.Table` zählt jedes gebuchte
Match, zwei Begegnungen desselben Paares landen beide darin. Es ist
ausschließlich der Spielplan, der zwei Treffen nicht auseinanderhalten kann.

Der Anlass ist ein realer Testlauf. Drei Spieler, jeder schlug einen und
verlor gegen den anderen — ein Ring. Die Tabelle löste ihn korrekt über die
Punktdifferenz auf (+1 / 0 / −1) und produzierte damit einen Sieger, der drei
Bälle mehr gemacht hatte. Das ist rechnerisch sauber und überzeugt an einer
Platte niemanden.

## Die Frage, die ADR-0009 offen ließ

ADR-0009 begründet, warum es keine `tournament_pairings`-Tabelle gibt:

> `position` ist das, was eine Auslosung reproduzierbar macht: das
> Kreisverfahren ist deterministisch über der Reihenfolge, die es bekommt, [...]
> und eine gespeicherte Kopie eines abgeleiteten Werts ist eine Kopie, die
> driften kann.

Ein Finale scheint dem zu widersprechen: seine Teilnehmer hängen am Ergebnis,
nicht an der Auslosung. Issue #132 hat es so formuliert — "die erste Paarung,
die sich nicht mehr aus `position` ausrechnen lässt".

**Das stimmt nicht.** Die Tabelle ist eine Funktion der gespeicherten Matches,
und die besten zwei sind eine Funktion der Tabelle. Das Finale ist damit
genauso ableitbar wie jede Runde des Kreisverfahrens — nur aus zwei Eingaben
statt aus einer: aus `position` **und** aus den Ergebnissen. Beide stehen in
der Datenbank.

Der Grundsatz aus ADR-0009 gilt hier also unverändert, statt eine Ausnahme zu
brauchen. Das ist der Grund, warum dieses ADR kürzer ausfällt als erwartet.

## Entscheidung

**Kein gespeicherter Spielplan. Ein Match sagt, welchen Platz es füllt.**

Eine additive Spalte `matches.tournament_round`:

- Gruppenspiele tragen ihre Runde aus dem Kreisverfahren. Bei Rückspielen
  bekommen Hin- und Rückrunde verschiedene Nummern, womit ein Paar pro Runde
  wieder eindeutig ist.
- Das Finale trägt die Nummer nach der letzten Gruppenrunde.
- Der Nachschlüssel im Spielplan wird `(Runde, Paar)` statt `Paar`.

Eine Tabelle für Paarungen entsteht nicht. Die Spalte speichert keinen
abgeleiteten Wert, sondern **welchen abgeleiteten Platz dieses Match füllt** —
das ist der Unterschied, an dem ADR-0009 hängt.

**Bestehende Zeilen bleiben `null`, und das ist kein Sonderweg.** In einem
einfachen Round Robin kommt jedes Paar genau einmal vor; dort sind `(Runde,
Paar)` und `(Paar)` dasselbe. Ein `null` lässt sich also exakt über das Paar
auflösen, nicht bloß notdürftig. Formate mit Wiederholungen gab es vorher
nicht, also kann keine Altzeile mehrdeutig sein.

**Zwei Formatangaben statt einer kombinatorischen.** `format` beschreibt die
Gruppe (`round_robin`, `double_round_robin`), ein Bool `with_final` sagt, ob
ein Endspiel folgt. Vier Namen für vier Kombinationen wären eine Liste, die
bei der nächsten Variante quadratisch wächst.

**Das Finale spielt, wer nach den Gruppenspielen auf Platz eins und zwei
steht**, sobald alle Gruppenspiele bestätigt sind. Vorher existiert der Platz
im Spielplan, aber ohne Namen.

**Das Finale steht nicht in der Gruppentabelle.** Sonst verschöbe sein
Ergebnis die Gruppenplätze und damit die Frage, wer im Finale hätte stehen
sollen — ein Kreis. Die Tabelle ist die Gruppe; das Finale entscheidet das
Turnier.

**Für die TTR ist es ein Match wie jedes andere.** ADR-0009 gilt unverändert:
einzeln gewertet, sofort bei der Bestätigung. Ein Endspiel zählt nicht doppelt,
weil es an einem Nachmittag nicht mehr wert ist als das Spiel davor.

**Ist die Tabelle an der Schnittkante echt gleich, gibt es kein Finale.** Die
Seite sagt das und die geteilten Plätze bleiben stehen. Ein Endspiel zwischen
zwei zufällig ausgewählten von drei Gleichplatzierten wäre keine Entscheidung,
sondern eine Auslosung mit Publikum. Der Fall ist seltener, als er klingt:
Punktdifferenz trennt fast immer, und genau deshalb ist sie ein Kriterium.

## Konsequenzen

**`tournament.Matches(n)` hört auf, `n*(n-1)/2` zu sein.** Die Zahl hängt jetzt
am Format und am Finale. Sie steht an drei Stellen in der Oberfläche —
Anlege-Formular, Liste, „x von y Spielen gewertet" — und muss überall dieselbe
Funktion benutzen.

**Ein Rückspiel-Turnier ist doppelt so lang.** Acht Leute sind dann 56 Matches
statt 28, bei einer Viertelstunde vierzehn Stunden. Die Obergrenze von zwölf
Spielern aus ADR-0009 war für den einfachen Fall gerechnet; für Rückspiele ist
sie zu hoch. Das Formular muss die Zahl zeigen, die tatsächlich entsteht, statt
der aus einem Format, das nicht gewählt wurde.

**Der Spielplan bekommt einen Zustand, den es nicht gab:** eine Paarung, deren
Platz feststeht und deren Namen noch nicht. Sie muss lesbar sein, ohne zu
behaupten, sie sei „offen" wie die anderen.

**Was nicht dazukommt.** Kein Halbfinale, kein Spiel um Platz drei, keine
K.-o.-Runde. Ein Feld von vier bis acht Leuten hat mit Gruppe plus Endspiel
seine Entscheidung; alles weitere ist ein Turniersystem, und dafür ist der
Auslöser aus #41 Schweizer System ab etwa zwölf Spielern, nicht ein Baum.

## Verworfene Alternativen

**Eine `tournament_pairings`-Tabelle.** Löst dasselbe Problem und würde auch
Formate tragen, die sich nicht ausrechnen lassen. Verworfen, weil sich genau
das hier nicht stellt: sowohl Rückspiele als auch das Finale sind ableitbar,
und eine gespeicherte Kopie eines ableitbaren Werts ist der Fehler, den
ADR-0009 benennt. Fällig wird sie, wenn eine Paarung entsteht, die niemand
ausrechnen kann — ein von Hand gesetztes Spiel etwa.

**Das Finale als eigenes, zweites Turnier**, angelegt wie „Nochmal ne Runde".
Kostet keine einzige Zeile Datenmodell. Verworfen, weil das Endspiel dann
nicht zum Turnier gehört, das es entscheidet: die Seite, auf der die Tabelle
steht, wüsste vom Sieger nichts.

**Den Ring über eine zusätzliche Regel auflösen** statt über ein Spiel — etwa
den direkten Vergleich der Beteiligten höher zu gewichten. Verworfen, weil
jede weitere Regel dasselbe Problem verschiebt statt es zu lösen: irgendwann
gewinnt jemand, weil eine Tabelle es sagt, und nicht, weil er gespielt hat.
Ein Endspiel ist die Antwort, die an einer Platte trägt.

**Das Finale automatisch für jedes Turnier.** Verworfen, weil eine
Mittagspause mit drei Spielen kein Endspiel braucht und die Frage beim Anlegen
einen Haken kostet, nicht mehr.
