# ADR-0034: Der Kiosk zählt mit, wenn kein Zählwerk da ist

- **Status:** accepted
- **Datum:** 2026-11-21
- **Betrifft:** Kiosk, Turniere, Oberfläche
- **Bezug:** ergänzt `0015-zaehlwerk-traegt-ergebnisse-ein` und
  `0028-turniere-ueber-das-zaehlwerk`: Punkt für Punkt zählt bisher nur das
  Zählwerk. `0013-turnierseiten-und-erster-aufschlag` gilt unverändert: Im
  Turnier schlägt Seite A zuerst auf, ausgespielt wird dort nicht.

## Kontext

Am 19. November haben vierzehn Leute im Kiosk-Modus gespielt, mit einem
Menschen als Schiedsrichter und ohne Zählwerk. Gezählt wurde im Kopf. Danach
wurden die Sätze am Kiosk eingetragen. Drei Dinge fehlten:

- **Ein Zähler im Turnier.** Die Kiosk-Turnierseite nimmt pro Paarung nur
  fertige Satzergebnisse an.
- **Einspielen und Ausspielen des Aufschlags („umdi“).** Gibt es nur in der
  Zählwerk-Oberfläche.
- **Wer gerade Aufschlag hat.** Der Schiedsrichter muss es im Kopf mitzählen:
  zwei Punkte (fünf bei 21), ab 10:10 jeder Punkt, im nächsten Satz der
  Verlierer des vorigen. Genau da passieren die Streitfälle.

Das Zählwerk kann das alles (zaehlwerk ADR-0005). Es braucht aber einen
eigenen Dienst, Taster oder ein zweites Gerät, und es war an diesem Abend
nicht da. Ein Abend ohne Zählwerk ist der Normalfall, für den es den Kiosk
gibt (ADR-0028: „Der Kiosk bleibt für den Abend ohne Zählwerk“).

## Entscheidung

**Die Kiosk-Turnierseite und die normale Kiosk-Eingabe bekommen einen Zähler,
und nur sie.
Er hält keinen Zustand auf dem Server, rechnet in einem eigenen Paket und füllt
am Ende die vorhandenen Satzfelder aus.**

### Vor dem ersten Punkt

Drei Fragen, mit den Vorgaben, die heute gelten:

- **Erster Aufschlag:** links, rechts oder **ausspielen** („umdi“). Beim
  Ausspielen tippt der Schiedsrichter auf die Seite, die den Ballwechsel
  gewonnen hat. Das zählt keinen Punkt. **Im Turnier wird nicht gefragt:**
  Seite A schlägt zuerst auf (ADR-0013), wie es im Spielplan steht.
- **Einspielen:** ja oder nein. Mit Einspielen steht „Einspielen“ groß da, bis
  der Schiedsrichter auf „Los“ tippt. Ein Spieltimer ist das nicht.
- **Seitenwechsel:** ja oder nein. Mit Seitenwechsel tauscht die Anzeige die
  Spieler nach jedem Satz und im Entscheidungssatz bei 5 (bei 21: bei 10), und
  zeigt „Seitenwechsel“, bis jemand weitertippt.

### Während des Spiels

- Zwei große Flächen, eine pro Spieler, auf der Seite, auf der er gerade
  steht. Tippen ist ein Punkt. Dazu „Rückgängig“.
- **Der Aufschlag wird angezeigt und mitgeführt:** ein deutlicher Marker bei
  dem, der aufschlägt, und darunter, wie viele Aufschläge er noch hat
  („noch 1“). Die Regel kommt aus dem Regelblatt: Wechsel nach
  `ServeEvery(pointsToWin)` Punkten, ab `DeuceFrom` nach jedem Punkt, im
  nächsten Satz beginnt der Verlierer des vorigen. Im Turnier eröffnet
  Seite A den ersten Satz (ADR-0013).
- Satzstand und gewonnene Sätze stehen daneben. Ein Satz endet von selbst bei
  `pointsToWin` mit zwei Punkten Abstand. Das Spiel endet, wenn einer die
  Mehrheit der Sätze aus `best_of` hat.

### Am Ende

Der Zähler füllt die Satzfelder des bestehenden Formulars aus (Kiosk oder
Turnierpaarung). Eingetragen wird erst mit dem Knopf, den es schon gibt, über
`handleKioskRecord` bzw. `handleTournamentRecord` mit derselben Prüfung
(`checkSlot`, Operator nach ADR-0014). Der Zähler trägt nichts selbst ein.

### Wie es gebaut wird

- **Ein Paket `internal/rally`** (Name offen) ohne Datenbank und HTTP:
  `State` (Sätze, laufender Satz, wer anfing, Phase, Seiten) und
  `Apply(state, event) State` für Punkt, Rückgängig, Aufschlag ausgespielt,
  Einspielen vorbei, Seitenwechsel bestätigt. Unit-Tests für jede Regel:
  Aufschlagfolge bei 11 und 21, Deuce, Satzwechsel, Entscheidungssatz,
  Rückgängig über einen Satz- und einen Seitenwechsel hinweg.
- **HTMX, kein eigenes JavaScript** (Invariante 7). Der Stand liegt als
  Hidden-Feld im Fragment, als Folge der Ereignisse und nicht als Ergebnis.
  Jeder Tipp schickt diese Folge plus das neue Ereignis an
  `POST /kiosk/counter`. Der Server spielt sie mit `Apply` ab und gibt das
  neu gezeichnete Fragment zurück.
  - Damit ist **Rückgängig** einfach das letzte Ereignis weglassen, auch über
    einen Satzwechsel hinweg.
  - Damit hält **der Server keinen Zustand**: kein Redis (Invariante 3), keine
    Tabelle, und es geht mit mehreren Replicas.
  - Die Route liegt unter `/kiosk`, damit sie das Kiosk-Cookie sieht, wie die
    Turnier-Eingabe.
- **Die Regeln gibt es dann zweimal**, hier und im Zählwerk. Das ist der Preis.
  Sie sind klein und stehen auf dem Regelblatt. Das Zählwerk zählt in Go in
  einem anderen Repository, und ein geteiltes Modul für dreißig Zeilen
  Aufschlagregel ist mehr Kopplung, als es spart.

## Warum nicht anders

**Nur vom Kiosk auf das Zählwerk verlinken.** Hilft nur, wenn ein Zählwerk
läuft. An dem Abend, der das hier ausgelöst hat, lief keins.

**Den Zähler in JavaScript im Browser.** Wäre ohne Netz flüssiger, ist aber
genau das handgeschriebene JS, das Invariante 7 nur erlaubt, wo HTMX nicht
reicht. Ein Tipp pro Punkt ist ein Request pro Ballwechsel, das trägt HTMX.

**Den laufenden Stand in der Datenbank.** Gäbe Reload-Sicherheit und eine
Live-Anzeige auf anderen Geräten, braucht aber eine Tabelle für etwas, das nach
zehn Minuten ein Ergebnis ist. Die Live-Anzeige hat das Zählwerk schon.

## Konsequenzen

- Am Kiosk lässt sich ein Spiel Punkt für Punkt zählen, mit Aufschlaganzeige,
  ohne dass jemand ein Handy braucht und ohne Zählwerk.
- Ein Reload der Seite verliert den laufenden Stand. Abhilfe später: den
  Ereignisstrom zusätzlich in die URL schreiben (`hx-push-url`).
- Das Regelblatt und der Zähler müssen dieselbe Aufschlagregel sagen. Beide
  lesen `ServeEvery` und `DeuceFrom`; die wandern dafür ins neue Paket.

## Was ausdrücklich wartet

- **Den Aufschlag speichern.** Der Zähler weiß, wer jeden Satz eröffnet hat
  und wer bei jedem Punkt aufschlug. Er zeigt es während des Spiels an,
  gespeichert wird weiter nur das Satzergebnis. Eine Spalte für den Eröffner
  pro Satz wäre additiv und kommt, wenn jemand eine Statistik „Punkte bei
  eigenem Aufschlag“ will.
- **Ausspielen im Turnier.** ADR-0013 bleibt: Bei achtundzwanzig Paarungen
  steht der Aufschlag im Spielplan.
- **Der Zähler auf dem Handy.** Erst der Kiosk. Die normale Eingabeseite
  bekommt ihn, wenn er sich dort bewährt hat.
