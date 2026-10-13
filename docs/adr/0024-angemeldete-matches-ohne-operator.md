# ADR-0024: Wer sich am Tisch angemeldet hat, braucht keinen Operator

- **Status:** accepted
- **Datum:** 2026-10-13
- **Betrifft:** Datenmodell, Schnittstellen, Messung
- **Bezug:** supersedes `0015-zaehlwerk-traegt-ergebnisse-ein` im Punkt „Der
  Operator ist Pflicht", und nur für Matches, zu denen sich beide Spieler am
  Tisch angemeldet haben. Alles andere in 0015 gilt weiter, ebenso
  `0014-kiosk-benennt-wer-eintraegt` für den Kiosk. Issue #321,
  stuttgart-things/zaehlwerk#92.

## Kontext

ADR-0015 lässt das Zählwerk Ergebnisse eintragen und verlangt dafür einen
Operator, der nicht mitspielt. Die Begründung kommt aus ADR-0014: Eine
Maschine hat keine Identität, also benennt sie den Menschen, der zusieht und
zählt. Der Operator ist dabei kein Beweis, nur ein Name aus einer Liste. Das
sagt ADR-0014 selbst: „Wer das Token hat, kann einen fremden Namen wählen."

Am Tisch kommt jetzt eine Anmeldung pro Tischseite dazu (zaehlwerk#92). Ein
Spieler meldet sich auf seiner Seite an, auf einem von zwei Wegen:

- **mit dem Handy**, über den NFC-Sticker oder QR-Code der Seite. Dann ist es
  sein angemeldeter Browser;
- **mit einem Schlüsselanhänger (Fob)** am Leser des Zählwerks, gefolgt von
  einem Druck auf den Taster seiner Seite (zaehlwerk-firmware#63). Wem der Fob
  gehört, weiß nur diese Anwendung (#321).

Sind beide Seiten belegt, startet das Zählwerk das Match mit genau diesen
beiden Spielern.

Danach steht kein dritter Mensch am Tisch, und es muss auch keiner dort
stehen. Die Pflicht aus ADR-0015 verlangte trotzdem einen. Dann wählt jemand
irgendeinen Namen, und `reported_by` behauptet einen Zuschauer, den es nicht
gab. Genau diese Art Eintrag hat ADR-0014 abgeschafft: einen, der in der
Datenbank anders aussieht, als es am Tisch war.

## Entscheidung

**Haben sich beide Spieler am Tisch angemeldet, nimmt `POST /api/results` das
Ergebnis ohne Operator an. Den Nachweis liefern die zwei Anmeldungen, und
diese Anwendung prüft sie selbst.**

### Die Anmeldung als Nachweis

- **Jede Anmeldung entsteht hier, nicht im Zählwerk.** Beim Handy-Weg sowieso.
  Beim Fob-Weg ist es die Abfrage, mit der das Zählwerk die UID auflöst (#321).
  Beide Wege legen eine Zeile mit Spieler, Tischseite, Weg (`phone` oder
  `rfid`) und Zeitpunkt an, und das Zählwerk bekommt deren Kennung zurück.
- **Das Ergebnis nennt die zwei Anmeldungen.** `POST /api/results` bekommt
  statt `operator_id` die Kennungen beider Anmeldungen. Angenommen wird nur,
  wenn:
  - es beide Anmeldungen gibt;
  - sie zu genau `home_id` und `away_id` gehören, auf verschiedenen Seiten;
  - keine von beiden schon ein Ergebnis getragen hat;
  - keine älter ist als die Gültigkeit einer Anmeldung (einige Stunden, eine
    Konstante im Code).
- **Eine Anmeldung trägt genau ein Ergebnis.** Ist das Match eingetragen, ist
  sie verbraucht. Ein zweites Ergebnis mit denselben Kennungen wird
  abgelehnt.
- So kann das Zählwerk keine zwei beliebigen Spieler als angemeldet ausgeben.
  Es kann nur weiterreichen, was diese Anwendung vorher selbst festgestellt
  hat.

### Im Datenmodell

- **`entered_via = 'checkin'`** als neuer Wert, mit erweitertem
  CHECK-Constraint wie in ADR-0015.
- **`reported_by` darf leer sein, aber nur bei `checkin`:**
  `check (reported_by is not null or entered_via = 'checkin')`. Bisher ist die
  Spalte `not null`. Gelockert wird nur eine Regel, keine Zeile ändert sich.
  Das ist vorwärtsgerichtet und additiv im Sinne von Invariante 8.
- **Die Anmeldungen in einer eigenen Tabelle**, und das Match verweist auf sie.
  Ein Match mit `entered_via = 'checkin'` zeigt dann auch Jahre später, wer
  sich wann wo angemeldet hat.

### Was bleibt

- **Jeder andere Weg braucht weiter einen Operator.** Ein Match, dessen
  Spieler auf der Seite des Zählwerks aus der Liste gewählt wurden, wird
  geschickt wie bisher, mit `operator_id`. Die Prüfung, dass der Operator
  nicht mitspielt, bleibt dort unverändert.
- **Das Ergebnis bleibt `pending`**, wie ADR-0015 es für die Testphase
  festlegt. Es gibt keinen Melder, also dürfen beide Spieler bestätigen. Die
  Regel dafür gibt es schon: `load` verlangt Teilnehmer und Nicht-Melder.
- **Der Kiosk ändert sich nicht.** ADR-0014 gilt dort unverändert. Am Kiosk
  meldet sich niemand an.

## Warum nicht anders

**Der Heimspieler als Melder.** Das spart die Lockerung der Spalte. Es ist
aber genau der Eintrag, den ADR-0014 entfernt hat, weil ein Abend dann aussah
wie Leute, die ihre eigenen Spiele melden. Die Messung zählte diese Spieler
als Melder.

**Ein Beobachterkonto „Zählwerk" als Melder.** Das braucht keine Migration an
`reported_by`, und ein Beobachter spielt nach ADR-0022 ohnehin nicht. Es macht
aber eine Maschine zu einem Konto, das in der Anmeldeauswahl steht und so
aussieht, als hätte ein Mensch zugesehen. ADR-0014 und ADR-0015 sagen beide,
dass eine Maschine keine Identität hat. Ein leeres `reported_by` sagt das
ehrlich.

**Dem Zählwerk glauben, dass angemeldet wurde.** Ein Feld `checked_in: true`
im Ergebnis wäre die kleinste Änderung. Es wäre aber nur eine Behauptung
hinter demselben Token, und das Token beweist nicht mehr, als der Kiosk-Token
in ADR-0014 bewiesen hat. Mit den Kennungen prüft diese Anwendung nur ihre
eigenen Einträge.

**Den Operator für alle Zählwerk-Matches abschaffen.** Ohne Anmeldung gibt es
nichts, was an seine Stelle tritt. Dann bliebe ein Ergebnis für zwei Namen,
die irgendjemand aus einer Liste gewählt hat.

## Konsequenzen

- **`reported_by` kann ab jetzt leer sein.** Wer über die Spalte auswertet,
  muss das kennen. Mit `count(distinct reported_by)` fallen diese Zeilen still
  heraus. Das ist richtig, sollte aber nicht zufällig so sein.
- **Die Definition of Done** in `scripts/definition-of-done.sql` weist
  `checkin`-Zeilen in einer eigenen Spalte aus, wie `scoreboard`, und zählt
  sie nicht ins Verdikt. Die Frage dort ist, ob Spieler **freiwillig und
  unerinnert** eintragen. Ein angemeldetes Match ist näher daran als ein
  Zählwerk-Match mit Operator, aber noch nicht dasselbe, solange das
  Bestätigen fehlt.
- **Zwei Migrationen**: der neue Wert für `entered_via` mit der gelockerten
  Spalte, und die Tabelle der Anmeldungen. Beide additiv.
- **Das Zählwerk braucht eine eigene Änderung** (zaehlwerk#92): Es schickt die
  Kennungen statt eines Operators und fragt nur dann nach einem Operator, wenn
  die Spieler aus der Liste kamen. Bis dahin ändert sich an seinem Verhalten
  nichts.
- `/api` wird um einen schreibenden Weg breiter, die Anmeldung. Er bleibt
  hinter demselben Token wie der Rest, und ohne Token gibt es ihn nicht.

## Was ausdrücklich wartet

- **Sofortige Wertung angemeldeter Matches.** Zwei Anmeldungen am Tisch sind
  vielleicht genug, um auf die Bestätigung zu verzichten. Das ist dieselbe
  offene Frage wie in ADR-0015 und wird mit ihr entschieden, nicht hier.
- **Der Fob als Identität.** Wie ein Fob gekoppelt, gespeichert und aufgelöst
  wird, regelt ein eigenes ADR (0025, #321). Dieses hier setzt nur voraus, dass
  eine Anmeldung per Fob hier entsteht.
- **Mehrere Tische.** Eine Tischseite braucht einen Tisch, sobald es mehr als
  einen gibt (#280, #60). Dann gehört der Tisch in die Anmeldung, und die
  Prüfung oben verlangt, dass beide Anmeldungen am selben Tisch stattfanden.
