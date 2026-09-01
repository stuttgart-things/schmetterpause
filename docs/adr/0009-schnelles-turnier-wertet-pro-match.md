# ADR-0009: Das schnelle Turnier wertet pro Match

- **Status:** accepted
- **Datum:** 2026-08-31
- **Betrifft:** Datenmodell, Wertung, Fachlogik
- **Bezug:** weicht bewusst von der *veranstaltungsweisen Wertung* in
  `CLAUDE.md` ab, beantwortet die offenen Punkte 1–3 aus Issue #41, ersetzt
  kein bestehendes ADR

## Kontext

`CLAUDE.md` legt unter „Fachliche Begriffe" fest:

> **Veranstaltungsweise Wertung** — bei Turnieren und Ligarunden werden die
> Erwartungswerte über alle Einzel eines Spielers summiert und einmal
> verrechnet, nicht nach jedem Match. Das macht das Ergebnis unabhängig von der
> Reihenfolge der Ergebniseingabe.

Die Rechenlogik dafür ist bereits da: `ttr.Rate(rating int, results ...Result)`
ist variadisch und summiert Erwartungswerte über beliebig viele Ergebnisse.
Was fehlt, ist der Weg dorthin — `scoring.Confirm` übergibt immer genau ein
Ergebnis, ein Match nach dem anderen.

Der Weg dorthin ist nicht klein, und der Grund steht im Schema:

```sql
create table ttr_history (
    match_id   uuid not null references matches (id) on delete cascade,
    ...
);
create unique index ttr_history_player_match_key on ttr_history (player_id, match_id);
```

`match_id` ist `not null`, und der Unique-Index bindet jede Historienzeile an
genau ein Match. Eine Verrechnung über 28 Matches hat kein einzelnes Match, auf
das sie zeigen könnte. Veranstaltungsweise Wertung braucht also mindestens:

1. eine Migration — `match_id` nullable, dazu ein `tournament_id` als zweiter
   Anker, und ein Constraint, das genau einen von beiden verlangt;
2. einen zweiten Settlement-Pfad neben `scoring.Confirm`;
3. eine Entscheidung, **wann** ein Turnier abrechnet — beim letzten
   bestätigten Match oder beim Schließen durch einen Admin (ADR-0008 nennt das
   Schließen bereits als Admin-Handlung);
4. eine Antwort darauf, was mit einem Turnier passiert, das nie fertig gespielt
   wird, und was die Rangliste in der Zwischenzeit anzeigt.

Gleichzeitig ist der Anlass für diese Änderung ein **schnelles Turnier**: vier
bis acht Leute, eine Platte, ein Nachmittag. Der Wunsch ist ein Spielplan und
eine Tabelle, nicht ein Wertungsverfahren.

## Entscheidung

**Ein Turniermatch wertet wie jedes andere Match: einzeln, sofort bei der
Bestätigung.** Ein Turnier ist eine Klammer um Matches — `matches.tournament_id`
—, kein eigener Wertungsvorgang.

Konkret:

- `internal/tournament` enthält **keine** Wertung. Es kennt den Spielplan
  (Round Robin nach dem Kreisverfahren) und die Tabelle (Siege,
  Direktvergleich, Satz- und Punktdifferenz) — beides ohne Datenbank und ohne
  HTTP, wie es die Konventionen für Fachlogik verlangen.
- `scoring.Confirm` bleibt unverändert. Ein Turniermatch durchläuft denselben
  Weg wie ein Feierabendspiel: eingetragen, bestätigt, gewertet.
- `ttr_history` bleibt unverändert. Keine Migration an der Historie.

## Konsequenzen

**Was es kostet.** Die Reihenfolge der Ergebniseingabe beeinflusst das
Endergebnis. Wer sein erstes Turniermatch gegen den Stärksten gewinnt und
danach spielt, steht anders da als wer dieselben Ergebnisse in umgekehrter
Reihenfolge einträgt. Genau dagegen existiert die veranstaltungsweise Wertung.

Die Größenordnung ist überschaubar, aber sie ist nicht null, und sie wächst mit
dem Feld: bei acht Spielern hat jeder sieben Einzel, deren Erwartungswerte sich
über den Abend gegenseitig verschieben.

**Warum das hier vertretbar ist.** Der Effekt verlangt, dass sich die TTR eines
Spielers *während* der Veranstaltung merklich bewegt. Bei einem Nachmittag mit
vier bis acht Leuten und Änderungskonstante 16 ist das eine Verschiebung im
einstelligen Bereich. Für eine Büro-Rangliste ist das unter der Auflösung, mit
der irgendjemand sie liest.

Dazu kommt: das gemischte Feld ist bereits akzeptiert. Matches im Modus
Best-of-3 bis 11 und Best-of-7 bis 21 zählen heute gleich viel für die TTR.
Eine Reihenfolgeabhängigkeit im einstelligen Bereich ist nicht die
unsauberste Näherung im System.

**Was offen bleibt und wann es fällig wird.** Die veranstaltungsweise Wertung
ist damit nicht verworfen, sondern vertagt. Der Auslöser, sie zu bauen, ist
**nicht** ein weiteres schnelles Turnier, sondern eines der beiden:

- eine **Ligarunde** — dort ist die Reihenfolgeunabhängigkeit keine Feinheit
  mehr, weil eine Runde über Wochen läuft und Ergebnisse in beliebiger
  Reihenfolge nachgetragen werden;
- ein Feld ab etwa **zwölf Spielern**, wo sich eine TTR über die Veranstaltung
  weit genug bewegt, dass die frühen Matches mit anderen Erwartungswerten
  gerechnet wurden als die späten.

Bis dahin bleibt `matches.tournament_id` die Klammer, die eine spätere
Neuberechnung überhaupt möglich macht: welche Matches zu welcher Veranstaltung
gehörten, steht dann schon in der Datenbank, auch für die Turniere, die vorher
gespielt wurden.

**Was das für die Messung bedeutet.** Nichts an dieser Entscheidung, aber es
gehört danebengeschrieben: ein Turnier kippt die Definition of Done aus
Issue #7. Acht Spieler „jeder gegen jeden" sind 28 Matches gegen eine Hürde von
zehn, und ein Spielplan ist genau die Erinnerung, die die Messung ausschließt.
`docs/turnier-vor-ort.md` sagt das bereits; `entered_via` sieht den Unterschied
zwischen Spieler- und Kiosk-Eingabe, aber nicht den zwischen freiwillig und
angehalten. Dafür gibt es `SINCE` in `task office:dod`.

## Verworfene Alternativen

**Veranstaltungsweise Wertung sofort mitbauen.** Korrekt nach `CLAUDE.md`, aber
es verschiebt den Aufwand von „Spielplan und Tabelle" auf „Migration der
Historie plus zweiter Settlement-Pfad plus Abschluss-Semantik". Der Anlass —
ein schnelles Turnier — trägt diesen Aufwand nicht, und eine halb gebaute
Verrechnung wäre schlechter als eine bewusst einfache.

**Turniermatches gar nicht werten, eigene Tabelle daneben.** Vermeidet den
Widerspruch vollständig und wäre ähnlich schnell zu bauen. Verworfen, weil die
Rangliste sich an einem Turnierabend dann nicht bewegt — und das ist genau der
Abend, an dem am meisten gespielt wird und am meisten hingesehen. Eine
Rangliste, die den größten Spieltag ignoriert, erklärt sich niemandem.

**Ein zweites Rating nur für Turniere.** Zwei Zahlen pro Spieler, die
auseinanderlaufen und beide erklärt werden müssen. `CLAUDE.md` deutet ohnehin
an, dass Turniermatches die normale TTR bewegen (Issue #41, offener Punkt 3).
