# ADR-0025: Ein Fob ist eine Identität, gekoppelt nur im Fenster

- **Status:** accepted
- **Datum:** 2026-10-14
- **Betrifft:** Datenmodell, Authentifizierung, Schnittstellen
- **Bezug:** erweitert `0003-identitaeten-eigene-tabelle` um den Provider
  `rfid`. Setzt `0024-angemeldete-matches-ohne-operator` für den Fob-Weg um.
  Issue #321, stuttgart-things/zaehlwerk#92 (die andere Seite derselben
  Abfrage, dort als ADR-0007 geplant), stuttgart-things/zaehlwerk-firmware#63.

## Kontext

Am Zählwerk meldet sich ein Spieler mit einem Schlüsselanhänger (Fob) an: Er
hält ihn an den RC522-Leser am Hub und drückt dann den Taster seiner
Tischseite (zaehlwerk-firmware#63). Das Zählwerk bekommt dabei nur die UID des
Fobs. Wem der Fob gehört, weiß es nicht und soll es nicht wissen
(zaehlwerk#92: das Zählwerk kennt keine Spieler). Diese Anwendung kennt die
Spieler. Sie muss die UID in einen Spieler übersetzen.

ADR-0024 setzt voraus, dass dabei eine Anmeldung entsteht, und zwar hier. Das
Ergebnis eines angemeldeten Matches nennt später genau diese Anmeldungen statt
eines Operators.

Drei Dinge machen den Fob heikler als ein Handy:

- **Es werden nur Fobs benutzt, keine Firmenausweise.** Das ist eine
  Vorgabe des Unternehmens. Hält jemand aus Versehen seinen Ausweis an den
  Leser, entsteht trotzdem eine UID. Eine Kartennummer, die außerhalb dieser
  Anwendung einen Ausweis identifiziert, gehört hier nicht in die Datenbank und
  nicht ins Log.
- **Eine UID ist kein Geheimnis.** Sie wird ohne jede Prüfung ausgelesen und
  lässt sich mit billiger Hardware auf einen anderen Chip kopieren. Ein Fob
  belegt, dass dieser Chip am Tisch war. Dass sein Besitzer am Tisch war,
  belegt er nicht.
- **Der UID-Raum ist klein**, vier oder sieben Byte. Ein einfacher Hash darüber
  lässt sich durch Ausprobieren umkehren.

## Entscheidung

**Ein Fob ist eine Zeile in `identities` mit `provider = 'rfid'`. Gespeichert
wird ein geschlüsselter Hash der UID, nie die UID. Gekoppelt wird nur in einem
kurzen Fenster, das der Spieler selbst öffnet. Jede erfolgreiche Auflösung legt
eine Anmeldung an.**

### Was gespeichert wird

- `subject = HMAC-SHA256(SP_RFID_UID_KEY, uid)`, hex-kodiert. Die UID wird
  vorher normalisiert (Großbuchstaben-Hex ohne Trenner), damit derselbe Fob
  nicht zwei Hashes ergibt.
- **`SP_RFID_UID_KEY` ist eine Umgebungsvariable** (Invariante 2) und liegt im
  Vault neben `SP_SESSION_KEY`. Ist sie nicht gesetzt, ist die Funktion aus:
  Die Route gibt es nicht, auf dem Profil gibt es keine Schaltfläche, und eine
  Logzeile beim Start sagt es.
- Ein Spieler darf mehrere Fobs haben, so wie mehrere Geräte (ADR-0003). Führt
  man zwei Spieler zusammen (ADR-0017), ziehen die Fobs mit den anderen
  Identitäten um.
- `identities` bekommt eine nullable Spalte `last_used_at`. Die
  Profilseite zeigt damit, wann ein Fob zuletzt benutzt wurde. Das ist additiv
  (Invariante 8) und gilt auch für die übrigen Provider, die sie leer lassen
  dürfen.

### Koppeln

1. Ein angemeldeter Spieler öffnet auf seinem Profil **„Fob koppeln"**. Damit
   ist für etwa 60 Sekunden ein Fenster offen.
2. Er hält den Fob an den Leser und drückt seinen Taster, also der normale
   Anmeldeweg am Tisch.
3. Das Zählwerk fragt nach der UID. Sie ist unbekannt, und ein Fenster ist
   offen. Also wird der Fob diesem Spieler zugeordnet. Die Antwort ist dieser
   Spieler, damit ist er gleichzeitig angemeldet.
4. Das Fenster schließt nach einem Fob oder nach Ablauf der Zeit. Die
   Profilseite zeigt den gekoppelten Fob.

Regeln dazu:

- **Es gibt höchstens ein offenes Fenster.** Wer eines öffnen will, während
  ein anderes offen ist, bekommt „bitte warten" und den Zeitpunkt, an dem es
  abläuft. So kann ein Fob nicht beim falschen Spieler landen.
- **Das Fenster liegt in Postgres**, in einer eigenen kleinen Tabelle mit
  Ablaufzeit. Es gibt genau eine Zeile, und ein Upsert überschreibt sie nur,
  wenn sie abgelaufen ist. Damit ist das auch bei mehr als einer Replica
  wettlauffrei, ohne Lock und ohne Redis. Auslöser 2 aus ADR-0002 betrifft die
  Buchung von Slots, nicht dieses Fenster. Invariante 3 bleibt unberührt.
- **Ein Fob, der schon jemandem gehört, wird nicht umgekoppelt.** Er löst zu
  seinem Besitzer auf, und das Fenster bleibt offen. Wer einen Fob
  weitergeben will, entfernt ihn zuerst auf seinem eigenen Profil.

### Auflösen

`POST /api/identities/rfid/resolve` hinter demselben Bearer-Token wie die
übrigen `/api`-Routen (ADR-0015, ADR-0023). Die UID steht im Body, nie im
Pfad, damit sie in keinem Zugriffslog landet. Mitgeschickt wird die
Tischseite, so wie das Zählwerk sie benennt (dort ADR-0005).

| Fall | Antwort | Was hier passiert |
|---|---|---|
| Fob bekannt | `200`, Spieler und Kennung der Anmeldung | Anmeldung angelegt, `last_used_at` gesetzt |
| Fob unbekannt, Fenster offen | `200`, wie oben | Fob gekoppelt, Fenster geschlossen, Anmeldung angelegt |
| Fob unbekannt, kein Fenster | `404` ohne Inhalt | **nichts** |

- **Eine unbekannte UID hinterlässt nichts.** Keine Zeile, keine UID und kein
  Hash im Log, in keiner Fehlermeldung, in keiner Antwort. Die Logzeile sagt
  „unknown fob" und sonst nichts.
- **Eine Anmeldung, die kein Ergebnis trägt, verfällt einfach** (ADR-0024).
  Schickt das Zählwerk eine Abfrage zweimal, etwa nach einem Timeout, entsteht
  eine zweite Anmeldung. Das schadet nicht: Nur eine von beiden wird ein
  Ergebnis tragen.
- Die Abfrage läuft über ein Repository-Interface (Invariante 5). Der Provider
  `rfid` steht im Auth-Paket neben den anderen (Invariante 4). Er ist aber
  **kein `Authenticator`**: Er meldet keinen Browser an. Er nennt einem
  vertrauenswürdigen Server einen Spieler.
- Die Antwort ist JSON. Das Zählwerk ist der externe Konsument, wie bei den
  anderen `/api`-Routen. `/api` wird damit um genau diese Route breiter, und
  ADR-0024 hat das angekündigt.

### Entfernen

Die Profilseite listet die Fobs eines Spielers mit Kopplungsdatum und letzter
Benutzung. Jeder lässt sich einzeln entfernen. Das löscht die Zeile in
`identities`. Anmeldungen und Ergebnisse, die schon entstanden sind, bleiben.
Sie verweisen auf den Spieler, nicht auf den Fob.

## Warum nicht anders

**Die UID im Klartext.** Am einfachsten zu bauen und zu debuggen. Aber eine
Ausweisnummer in der Datenbank identifiziert eine Karte auch außerhalb dieser
Anwendung, und über das Backup liegt sie dann auch in jedem Bucket, in den es
kopiert wird (stuttgart-things/stuttgart-things#2968).

**Ein einfacher Hash (SHA-256 ohne Schlüssel).** Er sieht aus wie Schutz und
ist keiner. Bei vier bis sieben Byte UID ist die Tabelle in Minuten
durchprobiert. Erst der Schlüssel macht den Hash nur hier vergleichbar.

**Koppeln ohne Fenster: eine unbekannte UID merken und später zuordnen.** Dann
läge jede UID, die je an den Leser kam, gespeichert, auch jeder versehentlich
gehaltene Firmenausweis. Genau das soll nicht passieren.

**Ein Admin trägt die UID von Hand ein.** Dafür müsste die UID irgendwo
angezeigt werden, im Log des Zählwerks oder auf einem Display. Dann ist sie
gespeichert und gelesen, bevor jemand entschieden hat, ob sie gekoppelt wird.

**Das Fenster im Speicher.** Es überlebt keinen Neustart, und bei zwei
Replicas sieht die eine nicht, was die andere geöffnet hat. Redis wäre das
passende Werkzeug, ist aber nach ADR-0002 nicht da, und eine Tabelle mit einer
Zeile reicht.

**Das Zählwerk kennt die Zuordnung Fob → Spieler selbst.** Das spart eine
Abfrage pro Anmeldung. Dann müsste das Zählwerk aber UIDs und Spieler
speichern. Das widerspricht seiner eigenen Regel, keine Spieler zu kennen, und
verteilt die Zuordnung auf zwei Systeme.

**Der Fob als Anmeldung im Browser.** Ein Fob am Leser des Kiosks könnte den
Spieler direkt anmelden. Da eine UID kein Geheimnis ist, wäre das eine
Anmeldung mit einem Passwort, das man aus der Tasche eines anderen auslesen
kann. Der Fob nennt einen Spieler für ein Match, das danach noch bestätigt
wird. Er öffnet kein Konto.

## Konsequenzen

- **Ein Fob ist ein schwächerer Nachweis als eine PIN.** ADR-0024 nimmt ihn
  trotzdem als Anmeldung, weil das Ergebnis `pending` bleibt und einer der
  beiden Spieler es bestätigen muss. Wird später über die sofortige Wertung
  angemeldeter Matches entschieden (ADR-0024, „Was ausdrücklich wartet"), muss
  diese Entscheidung den kopierbaren Fob ausdrücklich berücksichtigen.
- **`SP_RFID_UID_KEY` zu wechseln entkoppelt alle Fobs.** Die Hashes passen
  dann nicht mehr, und jeder Spieler muss neu koppeln. Es geht dabei nichts
  verloren außer den Kopplungen. Dasselbe gilt für einen Restore ohne den
  Schlüssel. Deshalb gehört er in Vault und in `docs/backup-restore.md` neben
  `SP_SESSION_KEY`.
- **Zwei Migrationen**, beide additiv: die Tabelle für das Fenster und
  `identities.last_used_at`. Die Tabelle der Anmeldungen kommt aus ADR-0024.
- **Eine neue `SP_*`-Variable.** kcl und Terraform ändern sich im selben
  Pull Request wie der Code, so wie beim Token für das Zählwerk.
- **Die Messung ändert sich nicht.** Koppeln und Auflösen sind keine
  Ergebnisse. Was ADR-0024 zur Definition of Done sagt, gilt für die Matches,
  die daraus entstehen.

## Was ausdrücklich wartet

- **Mehrere Tische.** Sobald es mehr als einen gibt (#280, #60), gehört das
  Fenster zu einem Tisch: eine Zeile pro Tisch statt einer. Die Anmeldung
  bekommt ihren Tisch dann über ADR-0024.
- **Andere Leser und andere Chips.** NTAG-Sticker für Handys sind ein eigener
  Weg (zaehlwerk#77) und laufen nicht über diese Route. Ein Fob mit
  kryptografischer Prüfung (etwa DESFire) würde die Schwäche oben beheben. Das
  ist eine andere Entscheidung mit anderer Hardware.
