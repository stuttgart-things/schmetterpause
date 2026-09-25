# ADR-0023: Das Zählwerk bietet Beobachter als Operator an

- **Status:** accepted
- **Datum:** 2026-09-25
- **Betrifft:** Schnittstellen
- **Bezug:** löst den ersten Punkt aus „Was ausdrücklich wartet" in
  `0022-beobachter-spielt-nicht` ein. Erweitert `0015-zaehlwerk-traegt-ergebnisse-ein`
  um einen lesenden Endpunkt, ersetzt es nicht. Issue #294.

## Kontext

ADR-0022 hat Beobachter eingeführt: Konten, die verwalten und nie spielen. Sie
fehlen in `GET /api/players`, weil diese Liste die zwei Seiten eines Matches
anbietet — und als Seite ist ein Beobachter ausgeschlossen.

`POST /api/results` nimmt einen Beobachter aber ausdrücklich als Operator an:
wer zusieht und zählt, ist genau das, was ADR-0014 einen Operator nennt. Das
Zählwerk füllt jedoch **alle drei** Auswahlfelder — links, rechts und wer zählt —
aus derselben Liste. Ein Beobachter lässt sich dort deshalb nie als Zähler
wählen, obwohl der Server ihn annähme.

ADR-0022 hat das bewusst liegen lassen, „bis jemand am Zählwerk als Beobachter
zählen will". Das ist jetzt der Fall: Im Büro soll `timoboll` zählen, ein Admin,
der nie spielt.

## Entscheidung

**Ein zweiter lesender Endpunkt, `GET /api/operators`: alle, die zählen dürfen.**

- Das sind **alle Konten**, Beobachter eingeschlossen, jedes mit
  `"observer": true|false`. Kein TTR: die Liste beantwortet, wer zählen darf,
  nicht wie jemand spielt.
- Dasselbe Token wie `/api/players`, und ohne gesetztes Token gibt es die Route
  nicht — wie jede andere unter `/api`.
- **`GET /api/players` bleibt unverändert** die Liste der möglichen Seiten.
- Ob der Operator im konkreten Match mitspielt, entscheidet weiterhin
  `POST /api/results`, gegen die zwei Seiten, die tatsächlich geschickt werden.
  Die neue Liste nimmt diese Prüfung nicht vorweg.

## Warum nicht anders

**Warum kein Feld an `/api/players`.** Das war die Richtung, die ADR-0022
skizziert hat: dem Vertrag ein Feld geben, das das Zählwerk auswerten muss.
Solange das Zählwerk das Feld nicht kennt, böte es einen Beobachter dann als
**Spieler** an, und jedes so gemeldete Match käme mit 422 zurück — die zwei
Repositories müssten gleichzeitig ausgerollt werden. Ein eigener Endpunkt lässt
jede Seite zuerst wechseln: ein älteres Zählwerk fragt ihn nie, ein neueres
fällt bei 404 auf `/api/players` zurück.

**Warum nicht nur die Beobachter.** Der Zähler ist oft ein Spieler, der gerade
nicht spielt. Eine Liste nur der Beobachter zwänge das Zählwerk, zwei Listen
zusammenzuführen, um eine Frage zu beantworten, die diese Anwendung selbst
beantworten kann.

**Warum `/api` damit nicht aufweicht.** ADR-0015 hält die Fläche so schmal wie
beschrieben. Das hier ist ein weiterer **lesender** Endpunkt für denselben
Konsumenten, hinter demselben Token, mit Daten, die `/api/players` bis auf das
Flag schon herausgibt. Er schreibt nichts und öffnet keinen neuen Konsumenten.

## Konsequenzen

- Die Fläche unter `/api` ist jetzt `players`, `operators` und `results`.
- Das Zählwerk braucht eine eigene Änderung, um die neue Liste für das
  Zählerfeld zu nutzen; bis dahin ändert sich an seinem Verhalten nichts.
- Ein Beobachter kann am Zählwerk zählen, sobald beide Seiten ausgerollt sind.
  `reported_by` benennt dann den Beobachter — den Menschen, der zugesehen hat,
  so wie ADR-0014 und ADR-0022 es wollen.
