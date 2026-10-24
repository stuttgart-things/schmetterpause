# ADR-0027: Eine Sandbox, in der Pong spielt, wertet sofort

- **Status:** proposed — Entwurf, nicht entschieden
- **Datum:** 2026-10-24
- **Betrifft:** Wertung, Betrieb, Schnittstellen
- **Bezug:** macht den in `0015-zaehlwerk-traegt-ergebnisse-ein` vorgesehenen
  Wechsel von `Create(pending)` auf `scoring.Record` pro Deployment wählbar,
  ohne ihn für das Büro zu entscheiden. Gegenstück zu zaehlwerk ADR-0015
  („Pong plays reported matches in a sandbox") und ADR-0016 („The sandbox on
  the office panel"). Ersetzt kein ADR.

## Kontext

Pong prüft heute das Zählwerk: Es weiß vor jedem Ballwechsel, wer ihn gewinnt,
und vergleicht das mit dem Spielstand. Für diese Anwendung gibt es nichts
Vergleichbares. Was ein Ergebnis hier bewegt — TTR, Rangliste, Statistik,
später Turniertabelle und Finale —, wird von Hand ausprobiert, ein Match nach
dem anderen.

Die Idee ist eine Sandbox: ein eigener Namespace mit einer eigenen
Schmetterpause, einem eigenen Zählwerk und Pong. Dort spielen Bot-Spieler mit
unterschiedlicher Stärke ein Match nach dem anderen, jedes Ergebnis kommt
hierher, und das Büro sieht die Spiele auf der Matrix. Auf der Seite des
Zählwerks regeln das dessen ADR-0015 und ADR-0016. Turniere über das Zählwerk
kommen später und werden auch am echten Tisch gebraucht; die Sandbox soll ihr
erster Nutzer sein.

Auf dieser Seite fehlen drei Dinge:

1. **Ein Zählwerk-Ergebnis wartet auf Bestätigung.** ADR-0015 legt es auf
   `pending`, und ADR-0024 lässt das für angemeldete Matches so. In der
   Sandbox bestätigt niemand. Die Matches stünden alle unter „Zu bestätigen",
   TTR und Rangliste bewegten sich nie, und eine spätere Turniertabelle käme
   nicht über die erste Runde hinaus, weil `tournament.Table` nur bestätigte
   Matches zählt.
2. **Es gibt keine Bot-Spieler.** Die Seed-Fixture aus Issue #82 legt sechs
   Kollegen mit zwölf Ergebnissen an. Sie ist für Previews gebaut, in denen
   jemand eine Seite beurteilt, nicht für eine Liga, die sich selbst füllt.
3. **Jedes Zählwerk-Ergebnis ohne Anmeldung braucht einen Operator**, der
   nicht mitspielt (ADR-0014, ADR-0015). In der Sandbox steht niemand am
   Tisch.

## Entscheidung

**`SP_SCOREBOARD_SETTLE` legt fest, ob ein Ergebnis über `POST /api/results`
wartet oder sofort wertet. Vorgabe ist `pending`, die Sandbox setzt
`immediate`. Dazu kommt eine zweite Seed-Fixture mit Bot-Spielern und einem
Beobachter, der für die Maschine steht.**

### Die Wertung

- **`pending`** ist das heutige Verhalten, unverändert: `Matches().Create` mit
  `domain.MatchPending`, beide Spieler dürfen bestätigen.
- **`immediate`** trägt über `scoring.Record` ein, so wie der Kiosk. Das
  Ergebnis ist sofort `confirmed`, TTR und `ttr_history` bewegen sich im selben
  Schritt. `entered_via` bleibt `scoreboard` bzw. `checkin`, `reported_by`
  bleibt der Operator bzw. leer. Die Herkunft ändert sich nicht, nur der
  Zeitpunkt der Wertung.
- **Beide Wege unter `/api/results`** folgen derselben Einstellung, mit und
  ohne Anmeldung. Zwei Schalter für dieselbe offene Frage aus ADR-0015 wären
  einer zu viel.
- **Ein anderer Wert hält den Start an**, wie jede ungültige Konfiguration.
  Beim Start steht in einer Log-Zeile, welche Wertung gilt. Bei `immediate`
  ist das eine Warnung, damit ein versehentlich gesetzter Wert im Büro im Log
  auffällt.

### Die Bot-Spieler

- **`SP_SEED_FIXTURE`** wählt, was `schmetterpause seed` anlegt: `demo`
  (Vorgabe, die heutige Fixture) oder `pong`. Die Sperre bleibt dieselbe:
  `seed` verweigert eine Datenbank, die schon Spieler hat (`ErrNotEmpty`).
- **`pong` legt acht Spieler mit festen Namen an und keine Ergebnisse.** Die
  Liga füllt sich selbst, und eine Rangliste, die bei 1000 für alle beginnt,
  zeigt am deutlichsten, ob sich der TTR nach Stärke sortiert.
- **Dazu ein Beobachter namens `Pong`** (ADR-0022). Er ist der Operator jedes
  Sandbox-Matches; das Zählwerk findet ihn über `/api/operators` (ADR-0023).
- **Die Stärke steht nicht hier.** Ein Bot ist für diese Anwendung ein
  Spieler wie jeder andere, ohne Flag und ohne Spalte. Wie gut er spielt,
  steht in Pongs Konfiguration (zaehlwerk ADR-0015), und ob er gut ist, soll
  der TTR herausfinden.

## Warum nicht anders

**Die Bots bestätigen lassen.** Prüfte zusätzlich den Bestätigungsweg. Dafür
müsste Pong sich als Spieler anmelden, mit PIN, Sitzungs-Cookie und
Formularen, die für einen Browser gebaut sind. Pong wäre damit ein zweiter
Client dieser Anwendung, der jede Änderung an der Anmeldung mittragen muss.
Für die erste Sandbox ist das zu viel. Es bleibt möglich, sobald jemand den
Bestätigungsweg unter Last sehen will.

**Sofortige Wertung an die Fixture koppeln.** „Wer mit `pong` geseedet ist,
wertet sofort" spart eine Variable. Es macht aber einen Datenbankinhalt zu
einem Schalter für das Laufzeitverhalten. Konfiguration geht nach Invariante 2
über Environment-Variablen, und eine Sandbox mit der Demo-Fixture soll genauso
sofort werten können.

**Ein Flag am Spieler, „Bot".** Damit ließen sich Bots ausblenden oder anders
werten. In der Sandbox gibt es aber nur Bots, und im Büro gibt es keine.
Ein Flag, das in keiner Instanz zwei Werte hat, unterscheidet nichts.

**Ein Beobachter als Melder widerspricht ADR-0024.** Tut er nicht. ADR-0024
verwirft ein Beobachterkonto „Zählwerk" für angemeldete Matches im Büro, weil
es so aussähe, als hätte ein Mensch zugesehen, und die Messung das mitzählte.
In der Sandbox gibt es weder Menschen noch Messung, und der Beobachter heißt
`Pong`. Er sagt, was er ist.

## Konsequenzen

- **Im Büro ändert sich nichts.** `SP_SCOREBOARD_SETTLE` ist dort nicht
  gesetzt, und `seed` läuft dort nicht.
- **Der Wechsel aus ADR-0015 wird zur Konfiguration.** Wenn die Testphase
  entschieden hat, dass Zählwerk-Ergebnisse sofort zählen dürfen, ist das im
  Büro eine Variable und kein Code mehr. Die Entscheidung selbst fällt weiter
  dort, nicht hier.
- **Ein versehentlich gesetztes `immediate` im Büro** wertet Zählwerk-Matches
  ohne Bestätigung, so wie der Kiosk. Das ist ärgerlich und lässt sich über
  das Fenster von `scoring.Undo` nur kurz zurücknehmen. Die Warnung beim Start
  ist der Schutz, den es dafür gibt.
- **Die Sandbox braucht ihr eigenes Rendering.** Der Seed-Job steht nach
  `docs/preview-environments.md` nur in `pr-*`-Artefakten. Eine Sandbox, die
  Release-Images fährt, bekommt deshalb ein eigenes KCL-Profil mit
  `seedEnabled` und `SP_SEED_FIXTURE=pong`, statt das Release-Artefakt zu
  patchen.
- **Die Namen der Bots verbinden zwei Repositories.** Pong ordnet die Stärke
  über den Anzeigenamen zu. Wer einen Bot hier umbenennt, ändert seine Stärke
  dort auf die Vorgabe. Das Zählwerk loggt das, es hält nichts an.
- Die Definition of Done betrifft die Sandbox nicht. Sie läuft gegen die
  Büro-Datenbank, und dort gibt es keine Bots.

## Was ausdrücklich wartet

- **Turniere über das Zählwerk.** Wer eine Paarung im Tableau vergibt und wie
  `tournament_id` über `/api/results` kommt, ist die in ADR-0015 offen
  gelassene Frage und bekommt ein eigenes ADR. Die Sandbox ist dann der Ort,
  an dem ein ganzes Turnier bis zum Finale durchläuft, bevor es am echten
  Tisch eins gibt.
- **Bots, die bestätigen.** Siehe oben.
- **Die sofortige Wertung im Büro.** Gehört zur Testphase aus ADR-0015.
