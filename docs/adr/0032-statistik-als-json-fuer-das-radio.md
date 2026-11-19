# ADR-0032: Die Statistik gibt es als JSON, für das Radio

- **Status:** accepted
- **Datum:** 2026-11-18
- **Betrifft:** Schnittstellen
- **Bezug:** erweitert die JSON-Oberfläche aus ADR-0015, die in
  `internal/server/api.go` „für alles Weitere ein eigenes ADR“ verlangt.
  Gegenstück: homerun2-radio-pitcher#6 (Segment `tabletennis-stats`).

## Kontext

Das LED-Panel im Büro spielt zwischen den Benachrichtigungen ein Programm, den
homerun2-radio-pitcher: Uhrzeit, Nachrichten, Zahlen aus dem Cluster. Ein
Segment soll erzählen, was an der Platte los war — wie viele Matches diese
Woche, der längste Satz, wer am häufigsten gewonnen hat.

Diese Zahlen rechnet diese Anwendung schon, für die Seite `/statistics`
(`stats.Compute`, `stats.Matrix`). Das Zählwerk kennt sie nicht: Es zählt ein
Match und gibt das Ergebnis ab, eine Liste führt es nicht. Ein Radio, das sie
selbst aus der Datenbank rechnet, wäre eine zweite Rechnung, die auseinanderläuft.

Für das Radio gilt außerdem: **Zahlen kommen nie vom Sprachmodell.** Es liest
sie aus einer Quelle und formuliert höchstens um. Die Quelle muss also genau
die Zahlen liefern, die auch die Seite zeigt.

## Entscheidung

**Ein lesender Endpunkt `GET /api/statistics`, hinter einem eigenen Token
`SP_STATISTICS_TOKEN`.**

- **`?days=N`** (1 bis 366) zählt die Matches der letzten N Tage, nach
  `played_at`. Ohne `days` zählt er alles, was die Seite auch zählt.
- **Dieselbe Rechnung wie die Seite:** `stats.Compute` über bestätigte Matches,
  mit derselben Obergrenze (`statisticsLimit`). Schneidet die Grenze ins
  Fenster, steht `truncated: true` in der Antwort — die Seite sagt dasselbe in
  Worten.
- **Felder:** `from` (fehlt ohne `days`), `until`, `truncated`, `matches`,
  `sets`, `points`, `deuce`, `whitewashes`, `longest_set` (`home`, `away`;
  fehlt, solange kein Satz gespielt ist) und `most_wins`: höchstens drei
  Spieler mit `id`, `display_name`, `won` und `lost`, die meisten Siege
  zuerst, bei Gleichstand weniger Niederlagen, dann die Reihenfolge der
  Rangliste. Wer im Fenster nicht gewonnen hat, steht nicht darin.
- **Nur Zahlen, keine Quoten** — aus demselben Grund wie die Seite (Issue
  #121).
- **Ein eigenes Token, nicht das des Zählwerks.** `SP_SCOREBOARD_TOKEN` darf
  Ergebnisse eintragen, und in der Sandbox zählen sie sofort (ADR-0027). Wer
  nur liest, soll nichts ändern können. Umgekehrt öffnet das neue Token keine
  andere Route.
- **Ohne Token gibt es die Route nicht**, wie bei Kiosk und Zählwerk.

## Was ausdrücklich nicht dazugehört

- Keine Liste der Matches und keine Matrix. Das Radio braucht Summen und ein
  paar Namen; jede weitere Form wäre eine, von der jemand abhängt.
- Keine Ratings und kein Verlauf. Die Rangliste bleibt auf der Seite.
- Kein Push. Das Radio fragt, wenn sein Sendeplan es will; diese Anwendung
  kennt das Radio nicht.

## Konsequenzen

- Die Invariante „keine JSON-APIs, solange kein externer Konsument existiert“
  bekommt einen zweiten Konsumenten. Sie gilt für alles andere weiter.
- `SP_STATISTICS_TOKEN` ist eine neue Variable (Invariante 2), im KCL-Modul als
  `statisticsEnabled` mit dem Vault-Schlüssel `statistics-token`.
- Die Namen der Spieler erscheinen auf dem Panel im Büro. Das ist dieselbe
  Öffentlichkeit wie die Startseite mit der Rangliste.
