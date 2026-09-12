# ADR-0017: Zwei Spieler zusammenführen heißt neu rechnen

- **Status:** proposed — Entwurf, nicht entschieden
- **Datum:** 2026-09-12
- **Betrifft:** Datenmodell, Wertung, Administration
- **Bezug:** löst den als "large, and probably its own ADR" markierten Teil von
  Issue #105, baut auf `0008-wer-fuer-andere-handeln-darf` auf, widerspricht
  keinem bestehenden ADR

## Kontext

Zwei Zeilen in `players` für dieselbe Person entstehen auf einem Weg, den
Issue #70 beschreibt: wer sein Cookie verliert, wird nicht wiedererkannt,
landet auf dem Beitrittsformular und legt sich ein zweites Mal an. Der
Wiederherstellungscode (ADR-0006) und die PIN (ADR-0007) haben das seltener
gemacht, nicht unmöglich — beide muss man haben, bevor man sie braucht.

An zwei Stellen steht heute geschrieben, dass es dagegen nichts gibt: in #70
selbst und in `docs/turnier-vor-ort.md`. Das ist die Wahrheit, die dieses ADR
ablösen soll.

### Warum das keine Fleißarbeit ist

Der naheliegende Griff — die `player_id` der Dublette überall auf den
Überlebenden umschreiben — scheitert an vier Stellen, und zwei davon würde die
Datenbank selbst abweisen. Alles hier am Schema nachgelesen, nicht vermutet:

| Was im Weg steht | Wo |
| --- | --- |
| `ttr_history_player_match_key`, eindeutig auf `(player_id, match_id)` | `20260821120000_init.sql:76` |
| `matches_players_differ check (home_id <> away_id)` | `20260821120000_init.sql:38` |
| `players_display_name_key`, eindeutig auf `lower(btrim(display_name))` | `20260821120000_init.sql:14` |
| `player_credentials`, Primärschlüssel `(player_id, kind)` | `20260828090000_player_credentials.sql:21` |
| `tournament_players`, Primärschlüssel `(tournament_id, player_id)` plus eindeutige Position | `20260831180000_tournaments.sql:50` |

Die erste und die zweite sind die interessanten. Haben die beiden **gegeneinander
gespielt**, wird aus dem Umschreiben ein Match gegen sich selbst, und das
verbietet der Check. Und für dieses Match trägt jede der beiden Zeilen einen
eigenen `ttr_history`-Eintrag — nach dem Umschreiben zwei für dieselbe Person
und dasselbe Match, was der eindeutige Index abweist.

### Die Stelle, die niemand sieht

Schwerer wiegt etwas, das keine Fehlermeldung erzeugt. Eine Wertung entsteht
in `scoring.go:263` aus **beiden Bewertungen zum Zeitpunkt des Matches**:

```go
homeChange, awayChange = ttr.RateMatch(home.TTR, away.TTR, outcome.HomeWon)
```

`ttr_history` hält deshalb `ttr_before` und `ttr_after` pro Spieler und Match —
eine Kette, in der jeder Wert aus dem vorherigen folgt. Führt man zwei Ketten
zu einer zusammen, folgt nichts mehr auseinander: Der zusammengeführte Spieler
hätte eine Historie, die nicht auf seine Bewertung hinausläuft.

Und es bleibt nicht bei ihm. **Jeder Gegner wurde gegen seine damalige
Bewertung gerechnet.** Ändert sich die, war jedes spätere Match dieses Gegners
aus einer Zahl gerechnet, die jetzt falsch ist. Der Fehler wandert durch die
ganze Tabelle.

## Entscheidung

**Zusammenführen ist kein Umschreiben, sondern eine Neuberechnung — und in dem
Fall, der tatsächlich vorkommt, ist gar nichts zu rechnen.** Zwei Stufen,
getrennt gehalten, weil die eine heute gebaut werden kann und die andere auf
einen Anlass wartet.

### Stufe 1: einer der beiden hat nichts gespielt

Der Normalfall aus #70, und der einzige, der ohne Arithmetik auskommt. Die
Dublette hat kein gewertetes Match — sie ist die Zeile, die entstand, während
die Person schon eine hatte.

Dann ist Zusammenführen genau das:

1. Die Identitäten der Dublette ziehen auf den Überlebenden um. Das ist keine
   Ausnahme, sondern wofür ADR-0003 die Tabelle vorgesehen hat: ein Spieler,
   mehrere Nachweise.
2. Wiederherstellungscode und PIN der Dublette werden **nicht** übernommen. Sie
   gehören zu einer Zeile, die verschwindet, und der Überlebende hat seine
   eigenen.
3. Schwebende und bestrittene Ergebnisse der Dublette ziehen mit um.
4. Ist die Dublette in einem Turnier, in dem der Überlebende **nicht** steht,
   zieht die Teilnahme um; stehen beide im selben Feld, wird abgewiesen — der
   Spielplan hätte danach ein Loch, und der Auslosung beizubringen, dass zwei
   Positionen eine sind, ist die größere Änderung.
5. Die Dublette wird entfernt. Das ist der Löschweg aus #234, und seine
   Fremdschlüssel sind das Geländer: bleibt irgendwo etwas hängen, weist die
   Datenbank ab, statt still etwas zu verlieren.

Keine Wertung bewegt sich, keine Kette wird angefasst, niemandes Bewertung
ändert sich. Der Anzeigename des Überlebenden bleibt; der Name der Dublette
wird wieder frei.

### Stufe 2: beide haben gespielt

Hier ist die Entscheidung getroffen, aber **nichts wird gebaut, bevor der Fall
das erste Mal wirklich eintritt** — dieselbe Form, die ADR-0002 für Redis
gewählt hat.

Wenn er eintritt, dann so:

- **Die ganze Tabelle wird neu gerechnet**, vom ersten Match an, in der
  Reihenfolge `confirmed_at`, bei Gleichstand `matches.id`. Nicht nur die Kette
  des zusammengeführten Spielers: eine Teil-Neuberechnung bricht die
  Eigenschaft, dass ein Match null Punkte erzeugt, und genau die macht die
  Rangliste zu einer Rangliste.
- **Matches der beiden gegeneinander werden gelöscht, nicht umgeschrieben.**
  Sind es dieselbe Person, hat dieses Match nie stattgefunden; es ist ein
  Eintragungsfehler, und die einzige ehrliche Behandlung ist, es zu entfernen.
- **Nichts darf offen sein.** Schwebende oder bestrittene Ergebnisse werden
  zuerst geklärt, sonst rechnet die Neuberechnung an etwas vorbei, das gleich
  dazukommt.
- **Alles in einer Transaktion.** Bei 54 Matches (#7) sind das Millisekunden;
  die Größenordnung ist auf Jahre hinaus kein Argument.

Dass dabei **fremde Bewertungen sich ändern**, ist kein Nebeneffekt, sondern
die Aussage: Wenn die beiden immer schon eine Person waren, ist die neu
gerechnete Historie die, die von Anfang an richtig gewesen wäre.

## Warum nicht anders

**Umetikettieren.** Die zwei Indexe oben weisen es ab, sobald die beiden je
gegeneinander gespielt haben — und wo sie es nicht tun, hinterlässt es eine
Historie, die nicht auf die Bewertung hinausläuft. Ein Verlauf, der sich nicht
nachrechnen lässt, ist schlimmer als keiner: `ttr_history` existiert laut der
Migration ausdrücklich dafür, eine fehlerhafte Rechnung nachvollziehbar zu
machen.

**Nur die Kette des Zusammengeführten neu rechnen.** Billiger, und es bricht
die Null-Summen-Eigenschaft. Issue #161 nennt sie beim Namen, als es um eine
höhere Änderungskonstante für Neulinge geht: sobald zwei Seiten eines Matches
unterschiedlich viel bewegen, addiert sich die Rangliste nicht mehr auf.

**Eine Alias-Tabelle, beide Zeilen bleiben stehen.** Damit stünden weiter zwei
Namen in der Rangliste, und die Frage "wer ist das" hätte zwei Antworten. Der
Ort für "zwei Nachweise, eine Person" ist `identities`, und den gibt es seit
ADR-0003 — ein zweiter daneben wäre eine zweite Vorstellung davon, was eine
Identität ist.

**Weiches Löschen der Dublette.** Eine als "weg" markierte Zeile, die weiter in
der Namensauswahl auftaucht, ist der Fehler, den das hier beseitigen soll.
Dieselbe Begründung wie beim Entfernen eines Spielers in #234.

## Konsequenzen

- **Stufe 1 ist klein und baut auf Vorhandenem auf**: Identitäten umhängen,
  dann der Löschweg aus #234. Kein neues Schema, keine neue Wertung.
- **Zwei Texte werden falsch, sobald Stufe 1 steht**: #70 und
  `docs/turnier-vor-ort.md` sagen beide, Zusammenführen sei nicht möglich. Das
  gehört im selben Pull Request geändert, sonst widerspricht die Dokumentation
  der Anwendung.
- **Ein Zusammenführen ist unumkehrbar.** Damit wird der offene Punkt 3 aus
  ADR-0008 — ob unumkehrbare Handlungen eine zweite Bestätigung brauchen —
  konkret, und dies ist der erste Fall, bei dem die Antwort wahrscheinlich "ja"
  lautet.
- **Eine Logzeile nennt beide `player_id` und den überlebenden Namen.** Nach
  dem Zusammenführen gibt es keine Zeile mehr, die sagt, was die andere war.
- **Der Blast Radius wächst über ADR-0004 hinaus**, wie schon beim Entfernen
  eines gewerteten Ergebnisses: Niemand bestätigt das. Deshalb gehört es hinter
  dieselbe Stufe wie alles andere in ADR-0008.
- **Stufe 2 bleibt bis auf Weiteres ein Absatz.** Wer sie baut, findet hier
  die Entscheidung vor und muss sie nicht neu treffen.

## Offene Punkte

Bewusst nicht entschieden, weil sie die Entscheidung oben nicht berühren:

1. **Welcher Name überlebt.** Der des Überlebenden ist die einfache Antwort,
   aber es ist gut denkbar, dass die Dublette den richtigen trägt — sie ist oft
   die neuere und sorgfältiger getippte. Neigung: die Handlung nimmt beide
   Spieler und den zu behaltenden Namen entgegen, statt ihn abzuleiten.
2. **Ob Stufe 2 einen Auslöser braucht oder eine Zahl.** Neigung: der erste
   echte Fall ist der Auslöser, so wie in ADR-0002.
3. **Ob die Neuberechnung aus Stufe 2 auch ohne Zusammenführen nützlich wäre**
   — als Werkzeug, das die Kette aus den Matches wiederherstellt, wenn sie je
   aus anderem Grund nicht mehr stimmt. Wahrscheinlich ja, und dann ist das
   Zusammenführen nur einer ihrer Aufrufer.
4. **Was mit einem Turnier geschieht, in dem beide stehen.** Stufe 1 weist ab.
   Ob das auf Dauer reicht, hängt daran, ob jemand eine Auslosung nachträglich
   verkleinern können soll — das ist eine Turnierfrage, keine Identitätsfrage.
