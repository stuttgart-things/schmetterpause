# ADR-0012: Ein Turnier kann ohne Wertung laufen

- **Status:** accepted
- **Datum:** 2026-09-03
- **Betrifft:** Turniere, Wertung, Datenmodell
- **Bezug:** schreibt `0009-schnelles-turnier-wertet-pro-match` fort. Ersetzt
  kein ADR: *wie* ein Turniermatch wertet, bleibt unverändert; hier kommt nur
  die Frage dazu, *ob* es das tut.

## Kontext

Im Büro gibt es zwei Sorten Turnier, und sie sehen im Formular gleich aus:

- die **Büromeisterschaft** — die zählt, darum spielt man sie;
- das **Spaßturnier** am Freitagnachmittag, halb im Feierabend, drei Bier
  in Reichweite. Wer da mitspielt, will nicht seine Rangliste riskieren.

Heute wertet jedes Turniermatch. Die Folge ist absehbar: wer seine Position
ernst nimmt, spielt beim Spaßturnier nicht mit — oder spielt mit und ärgert
sich hinterher. Beides ist schlechter als eine Frage beim Anlegen.

Es ist die einzige Turniereinstellung, deren Fehlen echten Schaden anrichtet.
Wertungsschlüssel (3/1/0 statt Siege), Rückrunde, Nichtantreten, Deadline —
alles nett, alles nachrüstbar, nichts davon macht ein Ergebnis falsch. Ein
Spaßturnier, das die Rangliste verstellt, macht sie falsch, und zwar
dauerhaft: die Änderung steht danach in `ttr_history` und lässt sich nur noch
über das Zehn-Minuten-Fenster von `scoring.Undo` zurücknehmen.

## Entscheidung

**Ein Turnier trägt ein Flag `rated`, Vorgabe `true`. Ist es `false`, bewegt
kein Match dieses Turniers eine Wertung.**

Genauer, und das ist der Teil, der später gegoogelt wird:

1. **Nicht gewertet heißt: die Zahl bewegt sich nicht.** `settle` überspringt
   `UpdateTTR` und `ttr_history` und setzt das Match trotzdem auf
   `confirmed`. Alles andere bleibt, wie es ist.
2. **Gespielt bleibt gespielt.** Das Match steht in der Matchliste, im
   Direktvergleich, in der Statistik und in der Bilanz der Rangliste. Nur die
   Zahl, nach der die Rangliste sortiert, rührt sich nicht. Ein zweiter
   Begriff von „zählt" — mal die Wertung, mal das Stattgefundenhaben — wäre
   der Anfang davon, dass jede Seite ihn anders auslegt.
3. **Die Turniertabelle merkt nichts davon.** `tournament.Table` zählt
   bestätigte Matches und keine Wertungen; ein Spaßturnier hat dieselbe
   Tabelle wie jedes andere.
4. **Die Entscheidung fällt einmal, beim Anlegen**, und gilt für das ganze
   Turnier. Sie ist änderbar, solange niemand gespielt hat — dieselbe Grenze,
   die `POST /tournaments/{id}/edit` schon für Modus und Feld zieht.

## Warum nicht anders

**Pro Match entscheiden.** Wäre flexibler und ist genau die Flexibilität, die
niemand will: die Frage käme achtundzwanzigmal statt einmal, und ein Abend,
in dem die Hälfte der Spiele zählt, ist nicht mehr erklärbar.

**Nachträglich entwerten.** „Das Turnier hätte nicht zählen sollen" wäre ein
Rückabwickeln über beliebig viele Matches, mit derselben Kette aus
`ErrNotLast`, an der schon `Undo` hängt: eine Wertung lässt sich nur
zurücknehmen, solange keine spätere darauf aufbaut. Nach einem Turnierabend
ist sie das nie. Die Frage muss deshalb vorher gestellt werden, und das
Formular stellt sie.

**Auch die Bilanz herauslassen.** Wäre konsequenter und wäre falsch: die
Statistikseite sagt, was an der Platte passiert ist, und Spiele daraus zu
verstecken, weil sie keine Wertung bewegt haben, macht aus einer Zählung eine
Auslegung. Wer wissen will, ob ein Match gewertet hat, sieht es in der
TTR-Spalte der Matchliste — dort steht dann ein Strich.

**Ein zweiter Turniertyp im Format-Feld.** `format` beschreibt die Form des
Spielplans (`round_robin`, `double_round_robin`). Ob gewertet wird, ist keine
Form. Vier Namen für vier Kombinationen wächst quadratisch, was `0011` schon
für das Finale entschieden hat.

## Konsequenzen

- Migration `20260903120000_tournament_rated.sql`: eine Spalte, `not null
  default true`, vorwärtsgerichtet und additiv. Jedes bestehende Turnier
  bleibt damit gewertet, was es ja auch war.
- `scoring.settle` lädt das Turnier, wenn ein Match eines hat. Ein Match ohne
  Turnier ist unverändert immer gewertet.
- `scoring.Undo` darf ein Match ohne Historienzeilen nicht mehr für kaputt
  halten. Bisher war „bestätigt, aber keine Historie" der Beweis, dass etwas
  anderes den Status geschrieben hat; jetzt ist es der Normalfall eines
  ungewerteten Turniers. Der `ErrNotLast`-Schutz entfällt dort mit — es gibt
  keine Kette, in die sich das Match eingereiht hätte.
- `Settlement` sagt, ob gewertet wurde, damit die Antwort nach der Eingabe
  nicht „±0" behauptet, wo nichts gerechnet wurde.
- Die Turnierseite und die Turnierliste sagen es dazu. Ein Turnier, dem man
  nicht ansieht, dass es nicht zählt, ist der Streit, den dieses ADR
  verhindern soll, nur später.

## Was ausdrücklich wartet

Nichtantreten/Walkover und eine Deadline, nach der ein Turnier sich selbst
beendet, sind erkannt und nicht gebaut. Sie werden gebaut, wenn ein Turnier
tatsächlich einmal offen hängen geblieben ist — vorher ist die Regel dafür
geraten, und eine geratene Regel über verfallene Spiele ist schwerer wieder
loszuwerden als eine fehlende.
