# ADR-0015: Das Zählwerk trägt Ergebnisse ein

- **Status:** accepted
- **Datum:** 2026-09-08
- **Betrifft:** Schnittstellen, Datenmodell, Messung, Betrieb
- **Bezug:** führt `0014-kiosk-benennt-wer-eintraegt` fort — das Zählwerk ist
  eine weitere Maschine ohne Identität, und die Antwort darauf ist dieselbe.
  Verbraucht die Invariante „keine JSON-APIs, solange kein externer Konsument
  existiert" aus `CLAUDE.md`, weil es ab hier einen gibt.

## Kontext

`zaehlwerk` zählt das laufende Spiel an der Platte: Punkte von Tastern, Piezos
oder einem Telefon, Satzlogik, Aufschlagwechsel, Undo. Sein ADR-0001 sieht die
Übergabe hierher von Anfang an vor — „Schmetterpause keeps players, TTR,
tournaments and history, and receives a finished result at match end" — und
`scorer.State.CompletedSets` trägt dort den Kommentar, dass es für genau diesen
Zweck existiert. Gebaut ist die Übergabe nie worden.

Zwei Richtungen fehlen. Das Zählwerk kennt Spieler nur als Freitext-Anzeigenamen
und braucht die echte Liste, damit ein Ergebnis überhaupt jemandem gehören kann.
Und das fertige Ergebnis muss hierher.

Beides scheitert heute an derselben Stelle: **diese Anwendung gibt nirgends JSON
aus.** Kein Handler in `internal/server/` ruft `json.Marshal` oder
`json.NewEncoder` auf, und das ist kein Versehen, sondern die Invariante. Sie
gilt aber ausdrücklich nur, „solange kein externer Konsument existiert".

Naheliegend wäre, den Kiosk wiederzuverwenden. `POST /kiosk/matches` tut fast
das Richtige: es trägt für zwei Spieler ein, die beide nicht der Eintragende
sind. Zwei Dinge passen nicht. Der Kiosk authentifiziert über Freischaltung,
Cookie und Grant — ein Sitzungstanz, der für einen Browser gedacht ist, an dem
abends jemand steht. Und `scoring.Record` ruft immer `settle`: ein Kiosk-Ergebnis
zählt sofort, ohne dass jemand zustimmt. Für eine Testphase ist das die falsche
Voreinstellung.

## Entscheidung

**Eine schmale JSON-Fläche für das Zählwerk, mit denselben Regeln, die für den
Kiosk gelten, und einem Ergebnis, das zunächst wartet.**

- **`GET /api/players`** gibt `id`, `display_name` und `ttr`. Gelesen, nicht
  gespiegelt: das Zählwerk hält keine Kopie, keinen Cache und gleicht nichts ab.
  Diese Anwendung bleibt der einzige Eigentümer der Spieleridentität.

- **`POST /api/results`** trägt ein Ergebnis ein, authentifiziert über ein
  eigenes Token statt über den Kiosk-Cookie. Ohne gesetztes Token gibt es die
  Routen nicht, so wie es den Kiosk ohne `SP_KIOSK_TOKEN` nicht gibt.

- **Der Operator ist Pflicht**, wie an jedem schreibenden Kiosk-Weg. Ein Aufruf
  ohne benannten Operator wird abgelehnt; `reported_by` ist der Operator; der
  Operator darf nicht mitspielen. ADR-0014 begründet das, und die Begründung
  gilt hier unverändert — eine Maschine hat keine Identität, also benennt sie
  den Menschen, der zusieht und zählt.

- **Das Ergebnis bleibt `pending`.** Nicht `scoring.Record`, sondern
  `Matches().Create(…, Status: domain.MatchPending)` wie der Spieler-Pfad. Weil
  der Melder ein Dritter ist, dürfen **beide** Spieler bestätigen: `load`
  verlangt Teilnehmer und nicht-Melder, und das sind hier zwei statt einem.

- **`entered_via` bekommt den Wert `scoreboard`**, per vorwärtsgerichteter
  Migration samt erweitertem CHECK-Constraint.

- **Die Definition of Done zählt diese Zeilen nicht ins Verdikt**, sondern zeigt
  sie als eigene Spalte neben `of which kiosk` und `of which tournament`.

## Warum nicht anders

**Warum nicht sofort werten, wie am Kiosk.** Die Testphase soll zeigen, dass die
Kette Taster → Zählwerk → Datenbank das Richtige einträgt. Solange das offen
ist, ist ein Ergebnis, dem ein Mensch zugestimmt hat, mehr wert als eines, das
schon in der Rangliste steht. Der Umstieg später ist der Wechsel von
`Create(pending)` auf `scoring.Record` — beide existieren, es ist keine
Umstellung, sondern eine andere Zeile.

**Warum die Zeilen nicht als `player` schreiben.** Das spart die Migration und
kostet die Herkunft, und die lässt sich nachträglich nicht rekonstruieren. Genau
diese Lücke hat Issue #71 geschlossen; sie wieder aufzureißen, um eine Migration
zu sparen, wäre ein schlechter Tausch.

**Warum sie nicht ins Verdikt zählen.** Die Messung fragt, ob Leute ihre
Ergebnisse **freiwillig und unerinnert** eintragen. Ein Zählwerk-Match ist
strukturell der Kiosk-Fall: eine Person zählt für andere. Entscheidend ist die
zweite Kennzahl — `reporters` ist `count(distinct m.reported_by)`, und mit dem
Operator als Melder hat ein ganzer Abend an der Platte genau einen. Die Hürde
„zehn Matches" fiele also, die Hürde „von fünf verschiedenen Spielern" bewegte
sich nicht. Das ist wörtlich die Verwässerung, vor der das Skript warnt: die
Messung besteht und beweist nichts.

Die Zeilen werden deshalb daneben ausgewiesen statt versteckt — „wieviel lief
übers Zählwerk" ist wissenswert, nur eben keine Antwort auf diese Frage. Die
Zählregel lässt sich jederzeit ändern, sobald die Frage eine andere ist; die
Provenienz nicht.

## Konsequenzen

- Diese Anwendung hat ab jetzt einen externen Konsumenten. Die Invariante, die
  JSON-Handler verboten hat, ist damit verbraucht — nicht aufgehoben: sie gilt
  weiter für alles, was keinen Konsumenten hat, und `/api` bleibt so schmal wie
  hier beschrieben.
- Ein Token mehr im Betrieb, mit denselben Eigenschaften wie das Kiosk-Token:
  nicht gesetzt heißt, die Fläche existiert nicht.
- Ergebnisse aus dem Zählwerk erscheinen erst in Rangliste und TTR, wenn einer
  der beiden Spieler zustimmt. In der Testphase ist das der Zweck; danach ist es
  eine offene Aufgabe und keine Eigenschaft.
- Ein Abend am Zählwerk ohne Bestätigung hinterlässt eine Reihe wartender
  Matches. `/fragments/pending` zeigt sie bereits.
- Die Migration erweitert einen CHECK-Constraint, statt Daten anzufassen. Nach
  Invariante 8 vorwärtsgerichtet und additiv.

## Was ausdrücklich wartet

- **Turniere über das Zählwerk.** `tournament_id` und `tournament_round` bleiben
  in diesem Weg leer. Ein Turnierspiel hat einen Platz im Tableau, und wer den
  vergibt, ist eine eigene Frage.
- **Sofortige Wertung.** Siehe oben; die Entscheidung dazu fällt, wenn die
  Testphase etwas gezeigt hat.
- **Ergebnisse zurück ins Zählwerk.** Die Fläche ist einseitig: Spieler heraus,
  Ergebnis herein. Ein Zählwerk, das den TTR-Stand anzeigt, braucht sie nicht.
