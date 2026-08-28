# ADR-0007: PIN als Anmeldung, Name zuerst

- **Status:** accepted
- **Datum:** 2026-08-28
- **Betrifft:** Authentifizierung, Datenmodell
- **Bezug:** schreibt `0006-wiederherstellungscode` fort und beantwortet dessen
  offenen Punkt 3; ersetzt es nicht

## Kontext

ADR-0006 hat den Wiederherstellungscode entschieden — ausdrücklich *keinen*
Login, sondern "der Weg, den man nur trifft, wenn etwas kaputtgegangen ist".
Das Cookie bleibt der Hauptweg.

Das reicht nicht. Zwei Gründe:

**Ein Notweg wird nicht als Anmeldung erkannt.** Wer ein neues Handy hat,
sucht eine Anmeldung. Ein Code, den man vor Wochen einmal angezeigt bekam,
ist zu diesem Zeitpunkt weder erinnert noch gesucht. Die Erwartung ist "ich
melde mich an", und was es dafür nicht gibt, existiert für den Nutzer nicht.

**Die Messung braucht Verlässlichkeit.** Der Lauf vom 26.08. ist an genau
dieser Lücke gescheitert (#70, #7): wer nicht mehr an seinen Spieler kam, trug
nichts mehr ein, und die Definition of Done liest das als Desinteresse. Der
Wiederholungslauf darf daran nicht ein zweites Mal scheitern.

Was heute technisch möglich ist, ist eng begrenzt. Die Anwendung läuft während
der Messung auf einem Laptop unter dem Tisch, über HTTP auf einer IP-Adresse:

- **Passkeys gehen nicht.** WebAuthn verlangt HTTPS, und die RP ID muss eine
  Domain sein — eine IP-Adresse ist als RP ID nicht zulässig. Auf
  `http://10.x.x.x:8080` gibt es kein WebAuthn, nicht schlecht, sondern gar
  nicht. Das ist eine harte Sperre, keine Abwägung.
- **SSO geht nicht.** Eine OAuth-Weiterleitung braucht eine feste
  Callback-URL; eine, die die heutige DHCP-Lease enthält, ist keine.

Bleibt ein geteiltes Geheimnis. Das ist keine Vorliebe, sondern das, was übrig
bleibt.

## Entscheidung

Eine **PIN** als zweite Art von Zugangsdaten, neben dem Wiederherstellungscode.

- **Optional.** Beim Beitritt angeboten und überspringbar, danach jederzeit aus
  der eigenen angemeldeten Sitzung setzbar.
- **Mindestens sechs Stellen, nur Ziffern**, Eingabefeld strikt numerisch.
- **Beide Arten liegen in einer Tabelle:**
  `player_credentials (player_id, kind, hash, updated_at)` mit
  `kind in ('recovery', 'pin')`, Hash mit **Argon2id**, Salt pro Zeile.
- **Anmeldung ist Name zuerst, Geheimnis danach** — für beide Arten, in einem
  Formular.
- **Begrenzung der Versuche pro Spieler und pro Adresse ist Bedingung**, nicht
  Nacharbeit: ohne sie wird die PIN nicht ausgeliefert.
- **Die PIN ersetzt den Code nicht.** PIN vergessen heißt Wiederherstellungs-
  code oder Kiosk.
- Der Code wird beim Beitritt weiterhin **automatisch** erzeugt und einmalig
  angezeigt.

## Begründung

### Warum PIN und nicht Passwort

ADR-0006 lehnt ein selbst gewähltes Geheimnis mit einem Argument ab, das steht:
ein Feld, in das man einen Code eintippen *kann*, wird ein Feld, in das jemand
sein **Firmenpasswort** tippt.

Die PIN entschärft das konkret statt nur zu hoffen: das Feld nimmt
ausschließlich Ziffern, `inputmode="numeric"`, und heißt PIN. In ein
Nur-Ziffern-Feld passt kein Firmenpasswort. Wer trotzdem sechs Ziffern
wiederverwendet, die anderswo etwas bedeuten, verliert dabei nichts, was über
die Sprengweite aus ADR-0004 hinausgeht.

Dazu der praktische Punkt: ein Passwort erzwingt einen Zurücksetzungsweg per
E-Mail. Wir haben keine E-Mail-Adressen und wollen keine.

### Warum nicht Passkeys zuerst

Nicht aus Abwägung, sondern weil es nicht geht: die RP ID muss eine Domain
sein. Passkeys sind der richtige Endzustand und werden die nächste Schicht,
sobald ein Hostname mit Zertifikat steht (#74). Dann sind sie billiger als
alles andere hier — kein Geheimnis auf dem Server, nichts zurückzusetzen, und
niemand muss etwas freigeben.

Ein Punkt, der dabei gern übersehen wird: **Passkeys brauchen selbst einen
Bootstrap.** Einen Passkey an einen *bestehenden* Spieler zu hängen, setzt
voraus, dass man beweisen kann, dieser Spieler zu sein. Der Code und die PIN
sind genau dieser Beweis. Sie sind also keine Verlegenheitslösung vor den
Passkeys, sondern deren Voraussetzung.

### Warum nicht SSO zuerst

Die feste Callback-URL fehlt heute. Dazu bleibt aus ADR-0006 die
Client-Registrierung als Vorgang bei Leuten mit anderen Prioritäten.

Die dortige Begründung "die Anwendung muss auf einem Netz hochkommen, das das
Internet nicht erreicht" trifft weiterhin den Laptop an der Platte. Für die
Anwendung auf dem Cluster hinter einem Gateway trifft sie nicht mehr — dort ist
SSO eine echte Option und als Admin-Identität ohnehin vorgesehen (#73).

### Warum Argon2id und eine eigene Tabelle

Damit ist der offene Punkt 3 aus ADR-0006 entschieden, und zwar durch die PIN.

Dort stand die Wahl zwischen einem gesalzenen Hash pro Zeile und einem
deterministischen Keyed Hash (HMAC mit dem Session-Key), der einen Index
erlaubt hätte. Für eine PIN ist die zweite Möglichkeit ausgeschlossen.

Solange der Schlüssel geheim bleibt, schützt ein HMAC auch einen kleinen
Wertebereich — vorberechnen kann nur, wer den Schlüssel hat. Das ist genau die
Annahme, die hier nicht trägt: Schlüssel und Datenbank liegen in derselben
Umgebung und landen im selben Backup. Wer das eine hat, hat meist auch das
andere. Und dann sind sechs Ziffern eine Million Werte, einmal durchgerechnet,
und die Tabelle gilt für alle Spieler gleichzeitig.

Argon2id hält dagegen auch nach einem Schlüsselverlust noch Kosten pro Rateversuch
aufrecht. Bei einem Geheimnis mit dieser Entropie ist das der ganze Unterschied
zwischen "unangenehm" und "sofort vorbei". Ein langsamer, gesalzener Hash ist
damit nicht die bequemere, sondern die einzige zulässige Wahl — und wenn es ihn
für die PIN braucht, wohnt der Code gleich mit darin.

Ein Nebeneffekt, der nach ADR-0006 nicht selbstverständlich war: ein Wechsel
von `SP_SESSION_KEY` entwertet die Zugangsdaten **nicht**. Bei der
HMAC-Variante wäre ein Schlüsselwechsel über alle Codes gegangen — zusätzlich
dazu, dass er ohnehin alle Cookies entwertet. Der Weg zurück überlebt jetzt
genau das Ereignis, für das man ihn braucht.

### Warum Name zuerst

ADR-0006 dachte den Code als "tippe den Code, wir finden dich". Mit Salt pro
Zeile erzwingt das einen Scan über alle Spieler, weil man ohne Kandidaten nicht
weiß, gegen welchen Salt zu prüfen ist — und mit Argon2id ist ein Scan nicht
nur unschön, sondern pro Versuch teuer.

Bei der PIN geht es ohnehin nicht anders. Wenn beide Wege gleich funktionieren,
verschwindet das Problem und die Oberfläche hat eine Form statt zwei: Namen
wählen, Geheimnis eingeben.

Die Spielerliste dabei preiszugeben kostet nichts. Die Rangliste ist öffentlich,
`/matches` nennt jeden Spieler, und der QR-Aushang hängt an der Wand.

### Warum optional

AP7 misst das Interaktionsbudget vom Scannen des Codes bis zum eingetragenen
Ergebnis. Eine Pflicht-PIN legt einen Schritt genau in den Weg, den die Messung
misst — und der Beitritt ist die Stelle, an der Leute abspringen.

Der Code wird deshalb weiter automatisch erzeugt: er kostet keine Interaktion,
nur eine Anzeige. Die PIN wird angeboten und darf übersprungen werden. Wer sie
setzt, hat einen Login; wer nicht, hat weiterhin Cookie und Code.

### Warum Rate-Limiting blockierend ist

ADR-0006 führt die Begrenzung unter "Nötig". Mit einer PIN wird sie zur
Auslieferungsbedingung.

Sechs Ziffern sind eine Million Möglichkeiten. Ohne Bremse ist die Länge das
Einzige, was zwischen jemandem und einem fremden Spieler steht — und dieselbe
Tür führt zum Wiederherstellungscode.

Die Sperre darf einen Spieler dabei **nicht dauerhaft aussperren**. Eine
Begrenzung, die das täte, würde #70 auf einem neuen Weg wiederherstellen.

## Konsequenzen

- **Positiv:** funktioniert heute über HTTP auf einer IP-Adresse. Kein neuer
  Dienst, keine Abhängigkeit, keine Freigabe von Dritten. Additiv zum Schema
  (Invariante 8), und weil Auth hinter einer Schnittstelle liegt
  (Invariante 4), ändert sich kein Handler.
- **Die PIN ist ein Bearer-Credential wie der Code.** Die Sprengweite bleibt die
  aus ADR-0004: ein falsches Ergebnis, das der Gegner bestätigen muss. Diese
  Grenze darf nicht überschritten werden.
- **Einschränkung: ein Verfahren mehr, das später überflüssig wird.** Sobald
  Passkeys stehen, ist die PIN das schwächste der drei. Sie dann wieder
  abzuschaffen ist eine eigene Entscheidung und wird Aufwand kosten — das ist
  der bewusst bezahlte Preis dafür, während der Messung eine Anmeldung zu haben.
- **Einschränkung: wiederverwendete PINs.** Dagegen hilft nur die Beschriftung
  und die Tatsache, dass hier nichts hängt, was sich zu stehlen lohnt.
- **Nötig:** die einmalige Anzeige des Codes muss unmissverständlich bleiben
  (ADR-0006). Eine zusätzlich angebotene PIN darf sie nicht zur Nebensache
  machen.

## Offene Punkte

1. **Form der Sperre.** Wachsende Wartezeit oder harte Sperre nach n Versuchen,
   und wie sie sich zurücksetzt. Neigung: wachsende Wartezeit pro Spieler,
   zusätzlich eine Obergrenze pro Adresse.
2. **Wann die PIN angeboten wird.** Direkt beim Beitritt neben dem Code, oder
   erst beim zweiten Besuch, wenn das Interaktionsbudget nicht mehr zählt.
3. **Darf der Kiosk eine PIN für jemanden setzen?** Neigung: nein. Der Kiosk
   stellt einen Code aus, die PIN setzt der Spieler selbst — sonst kennt sie
   zwischenzeitlich jemand anderes.
4. **Länge über sechs Stellen** — fest oder konfigurierbar.
5. **Was mit der PIN geschieht, wenn Passkeys kommen.** Bestehen lassen,
   auslaufen lassen oder abschaffen. Nicht jetzt zu entscheiden.
