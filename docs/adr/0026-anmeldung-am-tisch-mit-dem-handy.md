# ADR-0026: Anmeldung am Tisch mit dem Handy, über einen QR-Code pro Seite

- **Status:** accepted
- **Datum:** 2026-10-14
- **Betrifft:** Schnittstellen, Oberfläche, Konfiguration
- **Bezug:** setzt `0024-angemeldete-matches-ohne-operator` für den Handy-Weg
  um, wie `0025-fob-als-identitaet` es für den Fob getan hat. Issue #329,
  stuttgart-things/zaehlwerk#116 (die andere Seite),
  stuttgart-things/zaehlwerk#92.

## Kontext

ADR-0024 zählt zwei Wege als Nachweis, dass ein Spieler am Tisch steht: den
Fob am Leser und das eigene, angemeldete Handy. Der Fob-Weg ist gebaut (#321,
zaehlwerk#110). Für das Handy gibt es noch keine Route. ADR-0024 verlangt nur,
dass die Anmeldung **hier** entsteht und nicht im Zählwerk.

Am Fob-Weg fragt das Zählwerk: Es liest eine UID und will wissen, wem sie
gehört. Beim Handy ist es umgekehrt. Wer der Spieler ist, weiß diese Anwendung
bereits aus dem Cookie. Das Zählwerk weiß davon nichts, bis man es ihm sagt.

Ein NFC-Aufkleber auf der Handyhülle ist kein dritter Weg. Für den Leser ist er
ein Fob, und gekoppelt wird er wie einer (ADR-0025). Dafür braucht es nichts
Neues.

## Entscheidung

**An jeder Tischseite hängt ein QR-Code. Er öffnet eine Seite dieser
Anwendung. Dort bestätigt der angemeldete Spieler mit einem Knopf. Diese
Anwendung legt die Anmeldung an und meldet sie danach selbst ans Zählwerk.**

### Der Weg

1. Der QR-Code der linken Seite zeigt auf `/table/left`, der der rechten auf
   `/table/right`. Gedruckt wird er auf `/qr`, neben dem Code, den es dort
   schon gibt.
2. Die Seite braucht einen angemeldeten Spieler. Wer nicht angemeldet ist,
   meldet sich an oder tritt bei und landet danach wieder auf derselben Seite.
3. **Erst der Knopf meldet an, ein Aufruf des Links allein nicht.** Der Knopf
   schickt einen POST. Vorschauen in Messengern, ein Scan im Vorbeigehen oder
   ein zweiter Tab legen also nichts an.
4. Der POST legt eine Zeile in `checkins` an, mit `method = 'phone'` und der
   Seite. Danach ruft diese Anwendung das Zählwerk auf: „Spieler X mit Anmeldung
   Y belegt die linke Seite."
5. Die Seite zeigt die Antwort des Zählwerks in Worten: angemeldet, das Match
   startet, am Tisch läuft gerade ein Match, Zählwerk nicht erreichbar.

Ab da ist alles wie beim Fob. Sind beide Seiten belegt, startet das Zählwerk
das Match. Das Ergebnis kommt mit beiden Anmeldungen und ohne Operator zurück,
und `POST /api/results` prüft sie wie in ADR-0024. Eine Seite per Fob und die
andere per Handy ist ein gewöhnlicher Fall.

### Die Kopplung

- **Neu ist die Richtung.** Bisher ruft nur das Zählwerk hier an. Jetzt ruft
  diese Anwendung auch das Zählwerk auf, und zwar an genau einer Stelle: wenn
  ein Spieler den Knopf drückt.
- **`SP_ZAEHLWERK_URL` gibt es schon.** Über diese Adresse liest der Browser
  auf der Startseite den laufenden Spielstand. Dieselbe Adresse gilt jetzt
  auch für den Aufruf vom Server aus. Eine zweite Variable für dasselbe Gerät
  würde früher oder später auf ein anderes zeigen. Erreicht der Server die
  Adresse nicht, die der Browser benutzt, ist das ein Fehler in der
  Einrichtung, und die Seite sagt „Zählwerk nicht erreichbar".
- **Eingeschaltet ist es nur mit Adresse und Token zusammen.** Fehlt eines
  von beiden, gibt es `/table/*` nicht, `/qr` druckt keine Tischcodes, und
  beim Start steht eine Zeile im Log, die das sagt. So ist es auch bei
  `SP_SCOREBOARD_TOKEN` und `SP_RFID_UID_KEY` (Invariante 2).
- **Das Geheimnis ist das, das beide Seiten schon teilen.** Diese Anwendung
  schickt `SP_SCOREBOARD_TOKEN` als Bearer, also dasselbe Token wie bisher, nur
  in die andere Richtung. Ein zweites Geheimnis müsste man verteilen, im Vault
  ablegen und rotieren, und es würde nichts schützen, was das erste nicht
  schützt: Wer es hat, kann ohnehin schon Ergebnisse eintragen.
- **Der Aufruf wartet kurz und wird nicht wiederholt.** Antwortet das
  Zählwerk nicht, sagt die Seite das, und der Spieler drückt noch einmal.
  Eine Schleife, die im Hintergrund weiterversucht, würde einen Spieler
  anmelden, der längst gegangen ist.
- **Das Zählwerk prüft nicht nach, und das muss es auch nicht.** Es bekommt
  Spieler und Anmeldung von der Stelle, die beide angelegt hat. Ob die Angaben
  stimmen, prüft `POST /api/results` ohnehin noch einmal, bevor ein Ergebnis
  angenommen wird.

### Was bleibt, wenn es schiefgeht

- **Das Zählwerk lehnt ab oder ist nicht erreichbar:** Die Anmeldung bleibt
  als unbenutzte Zeile stehen. Allein beweist sie nichts, nach drei Stunden
  trägt sie kein Ergebnis mehr (`checkin.Validity`), und aufgeräumt werden
  muss nichts. Eine Zeile zu löschen, die einmal angelegt wurde, würde die
  Spur entfernen, dass jemand es versucht hat.
- **Ein Beobachter** meldet sich nicht an (ADR-0022). Die Seite sagt das, und
  es entsteht keine Zeile.
- **Zweimal tippen:** Pro Spieler gilt ein Ratenlimit. Doppelt angelegte
  Zeilen schaden aber auch nicht, denn das Zählwerk ersetzt eine Anmeldung
  desselben Spielers auf derselben Seite durch die neuere.

## Warum nicht anders

**Der QR-Code öffnet eine Seite des Zählwerks.** Das Zählwerk kennt keine
Spieler und hat keine Sitzung. Es müsste den Spieler hierher schicken und
wieder zurückholen. Die Anmeldung entstünde dann dort, wo ADR-0024 sie nicht
haben will.

**Das Handy spricht selbst mit dem Zählwerk.** Dann bräuchte das Handy ein
Geheimnis für das Zählwerk, oder das Zählwerk würde jedem glauben, der eine
Spieler-ID schickt. Beides ist schlechter als ein Aufruf zwischen zwei
Servern, die einander schon kennen.

**Ein wechselnder Code auf dem Panel des Zählwerks** statt eines festen
QR-Codes. Wer den festen Link kennt, kann sich auch von außerhalb des Raums
anmelden. Dagegen hilft nur ein Code, der sich ändert und nur am Tisch zu
sehen ist. Das ist aber ein eigenes Stück Arbeit im Zählwerk und auf dem
Panel. Der Schaden, den es verhindert, ist klein:

- Man kann **nur sich selbst** anmelden, nie einen Kollegen.
- Ein Match startet erst, wenn **auch der Gegner** angemeldet ist.
- Eine einzelne Anmeldung verfällt im Zählwerk nach wenigen Minuten.

Der wechselnde Code kommt, wenn sich zeigt, dass jemand den festen Link
missbraucht.

**Ein GET meldet an.** Das spart einen Knopf. Ein Link, der schon beim
Aufrufen etwas anlegt, wird aber auch von Vorschauen, Prefetch und dem
Verlauf des Browsers ausgelöst.

## Konsequenzen

- **Keine neue Variable.** `SP_ZAEHLWERK_URL` und `SP_SCOREBOARD_TOKEN` gibt
  es schon, in Code, kcl, Terraform und Compose. Neu ist, dass beide zusammen
  die Tischseiten einschalten. Das gehört in `docs/deployment.md` und in
  `kcl/README.md`.
- **`/qr` bekommt zwei Codes**, je einen pro Seite, zum Ausdrucken und
  Aufkleben.
- **Das Zählwerk braucht eine Route**, über die diese Anwendung eine Seite
  belegt (zaehlwerk#116). Ihre genaue Form legt das Zählwerk in einer eigenen
  ADR fest. Diese Anwendung schickt Seite, Spieler-ID und Anmeldungs-ID, und
  der Rest folgt dem Fob-Weg.
- **Die Anmeldung kann jetzt von zwei Wegen kommen.** `checkins.method` kennt
  `phone` schon seit der Migration für den Fob, also braucht es keine neue.

## Was ausdrücklich wartet

- **Der wechselnde Code** (siehe oben).
- **Mehrere Tische** (#280): Dann trägt der Pfad den Tisch, also
  `/table/{tisch}/left`, und die Anmeldung trägt ihn auch.
- **Name und PIN am Touch-Display** (harvester#432). ADR-0024 zählt das als
  Nachweis, aber es ist ein anderes Gerät und bekommt eine eigene
  Entscheidung.
