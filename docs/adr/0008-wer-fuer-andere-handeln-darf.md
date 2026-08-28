# ADR-0008: Wer für andere handeln darf

- **Status:** proposed — Vorschlag, nicht entschieden
- **Datum:** 2026-08-28
- **Betrifft:** Authentifizierung, Datenmodell, Betrieb
- **Bezug:** beantwortet den offenen Punkt 5 aus `0006-wiederherstellungscode`,
  baut auf `0007-pin-als-anmeldung` auf, ersetzt keins von beiden

## Kontext

**Diese Anwendung hat keine Administration.** Das Einzige mit erhöhten Rechten
ist der Kiosk, und ihn "Admin" zu nennen schmeichelt ihm (Issue #73):

- sein Cookie ist eine **Konstante** — `HMAC(Session-Key, "kiosk:" + Token)` —,
  also in jedem Browser identisch, der die Token-Adresse je geöffnet hat;
- zwölf Stunden gültig;
- zurücknehmen lässt er sich nur, indem man das Token ändert und **neu
  startet**, was den Laptop an der Platte mit aussperrt;
- und es gibt keine Aufzeichnung, wer damit was getan hat (Issue #77).

Für seine heutige Aufgabe — eine Maschine am Tisch trägt für alle Ergebnisse
ein — ist das angemessen, und ADR-0004 begründet das. Als Fundament für mehr
ist es dünn.

Phase 2 stützt sich trotzdem schon darauf. ADR-0006 macht den Kiosk zu der
Stelle, die einen Wiederherstellungscode für **jemand anderen** ausstellt, und
genau das ist inzwischen gebaut. Eine administrative Fähigkeit über fremde
Identitäten, gehalten von einer geteilten konstanten Zeichenkette, ohne Spur,
ist etwas anderes als die Fähigkeit, ein Satzergebnis einzutippen.

Und Phase 2 will mehr, was heute nirgends zu Hause ist:

| Handlung | Woher sie kommt |
| --- | --- |
| Zwei Spieler zusammenführen, die dieselbe Person sind | #70, `docs/turnier-vor-ort.md` |
| Ein gewertetes Ergebnis korrigieren oder entfernen | bisher nur das Zehn-Minuten-Undo am Kiosk (#49) |
| Einen Scherzspieler entfernen, oder einen doppelt angelegten | nichts |
| Ein Turnier schließen, entscheiden wann eine Veranstaltung abrechnet | #41 |

Der letzte Punkt wiegt schwerer als er aussieht: veranstaltungsweise Wertung
heißt, dass jemand sagen muss "dieses Turnier ist vorbei, rechne ab". Das ist
eine Entscheidung, keine Berechnung, und sie braucht einen Eigentümer.

**Was sich seit ADR-0006 geändert hat, ist die Voraussetzung.** Damals konnte
"wer darf das" an gar keine Person gebunden werden, nur an einen Browser — es
gab kein Mittel, mit dem jemand nachweist, wer er ist. Mit dem
Wiederherstellungscode und der PIN gibt es das jetzt. Das ist der Grund, warum
diese Entscheidung heute fällt und nicht in ADR-0006 fallen konnte.

## Entscheidung

**Zwei Arten von Vertrauen, getrennt gehalten, weil sie aus verschiedenen
Quellen kommen.**

### Der Kiosk behält, was er gut kann: Vertrauen aus dem Raum

Er darf genau zwei Dinge, und beide sind an die Anwesenheit gebunden:

- Ergebnisse für beliebige Spieler eintragen, samt dem Zehn-Minuten-Undo.
- Einen Wiederherstellungscode für jemanden ausstellen, der **daneben steht**.

Mehr nicht. Insbesondere niemals: ein gewertetes Ergebnis ändern oder löschen,
Spieler zusammenführen, Spieler entfernen, ein Turnier abrechnen.

### Alles mit bleibenden Folgen braucht ein benanntes Konto

Ein Flag am Spieler: `players.is_admin`. Wer es hat, handelt in seiner **eigenen
angemeldeten Sitzung**, und jede solche Handlung wird mit seiner `player_id`
protokolliert.

- **Der erste Admin kommt aus der Umgebung**: `SP_BOOTSTRAP_ADMIN` nennt einen
  Anzeigenamen, und beim Start bekommt dieser Spieler das Flag. Danach vergeben
  Admins es untereinander. Kein `psql`, und Invariante 2 bleibt gewahrt.
- **Ein Flag, keine Rollen.** Eine Stufe. Wenn je eine zweite gebraucht wird,
  ist das eine eigene Entscheidung.
- **Eine PIN darf niemand für jemand anderen setzen** — auch kein Admin. Das
  steht schon in ADR-0007 und wird hier nicht aufgeweicht.

### Damit beantwortet sich #77 von selbst

Kiosk = Ergebniseingabe und der Code für jemanden im Raum. Admin = Identität
und Historie. Das ist Möglichkeit 3 aus Issue #77, und #73 argumentiert bereits
in diese Richtung.

## Begründung

### Warum ein Flag am Spieler und nicht der Kiosk

Der Kiosk hat eine echte Eigenschaft, die kein Konto hat: **das Vertrauen kommt
aus dem Raum.** Jemand steht am Tisch, und die Leute kennen sich. Für "gib der
Person vor mir einen neuen Code" ist das nicht nur ausreichend, es ist besser
als jedes Passwort.

Für alles andere ist es das Falsche. Ein Ergebnis, das vor drei Wochen gewertet
wurde, zu entfernen, hat mit dem Raum nichts zu tun — niemand steht daneben,
und niemand sieht es. Die zwei Eigenschaften, die dabei zählen, fehlen dem
Kiosk beide:

- **Rücknehmbar.** Ein Flag knipst man aus. Ein Kiosk-Cookie nicht, ohne alle
  auszusperren und neu zu starten.
- **Nachvollziehbar.** Eine `player_id` im Log benennt einen Menschen. "Der
  Kiosk" benennt einen Browser, und davon gibt es beliebig viele mit demselben
  Wert.

### Warum jetzt und nicht in ADR-0006

Weil die Voraussetzung fehlte. Ein Flag an einer Person ist wertlos, solange
sich niemand als diese Person ausweisen kann — es wäre ein Flag an einem
Cookie gewesen, also wieder der Kiosk mit mehr Schritten. Code und PIN sind
genau dieser Ausweis, und sie stehen erst seit ADR-0007.

Das ist dieselbe Beobachtung wie bei den Passkeys: sie brauchen einen
Bootstrap, und Code und PIN sind er. Hier auch.

### Warum nicht OIDC-Gruppen zuerst

Das ist die richtige Antwort für eine Firmenanwendung und bleibt der Endzustand.
ADR-0006 sagt, warum sie nicht zuerst kommt, und das gilt für den Laptop unter
dem Tisch unverändert: ohne Internet meldet sich niemand an, und eine feste
Callback-URL, die die heutige DHCP-Lease enthält, ist keine.

Für die Anwendung auf dem Cluster hinter einem Gateway trifft das nicht mehr zu.
Dort wird SSO eine echte Option — und dann ersetzt Gruppenmitgliedschaft dieses
Flag, oder OIDC weist nur die Identität nach und das Flag bleibt. Das ist
offener Punkt 4 und braucht #89 zuerst.

### Warum der Bootstrap eine Umgebungsvariable ist

Issue #73 nennt als Preis des Flags "einen Weg, es zu vergeben, der nicht `psql`
ist". Eine Umgebungsvariable ist dieser Weg und die einzige Form, die zu
Invariante 2 passt: keine Config-Datei im Image, kein Handgriff in der
Datenbank, und dasselbe Image in Compose, Kubernetes und Azure Container Apps.

Sie wird bei jedem Start ausgewertet, nicht nur beim ersten. Das ist Absicht:
wenn sich jemand selbst aussperrt, ist der Weg zurück ein Neustart mit gesetzter
Variable und nicht ein Datenbank-Werkzeug.

### Warum der Kiosk nicht einfach abgeschafft wird

Weil er das Einzige ist, was während der Messwoche an der Platte funktioniert.
Ein Turnierabend läuft über eine Maschine mit einem Zettel, und diese Maschine
soll gerade **nicht** die Sitzung eines Spielers halten — acht Spieler von einem
Laptop einzutragen darf den Laptop nicht als den achten angemeldet
zurücklassen.

## Konsequenzen

- **Positiv: der Kiosk hört auf, die Obergrenze zu sein.** Sein konstantes
  Cookie ist wieder verhältnismäßig, sobald nichts Unumkehrbares mehr daran
  hängt. Das entschärft #77, ohne am Kiosk etwas zu ändern.
- **Positiv: additiv.** Ein `boolean`-Flag mit Default `false` (Invariante 8),
  und weil Auth hinter einer Schnittstelle liegt (Invariante 4), ändert sich
  kein bestehender Handler.
- **Die Sprengweite wächst zum ersten Mal über ADR-0004 hinaus.** Bisher war das
  Schlimmste ein falsches Ergebnis, das der Gegner bestätigen muss. Ein Admin
  kann ein gewertetes Ergebnis entfernen, und das bestätigt niemand. Das ist
  der bewusst bezahlte Preis dafür, dass jemand Fehler korrigieren kann — und
  der Grund, warum das Flag sehr wenige Leute haben und jede Handlung damit
  im Log steht.
- **Einschränkung: ein Admin hängt an einer sechsstelligen PIN.** Die
  Begrenzung aus ADR-0007 hält Raten auf, aber eine geratene PIN kostet hier
  mehr als anderswo. Siehe offener Punkt 2.
- **Einschränkung: `SP_BOOTSTRAP_ADMIN` nennt einen Anzeigenamen**, und Namen
  sind eindeutig, aber änderbar. Wer sich umbenennt und dann neu startet,
  vergibt das Flag an niemanden — oder, schlimmer, an jemand anderen, der den
  alten Namen inzwischen genommen hat. Das ist ein Grund, das Flag nicht nur
  aus der Variable zu beziehen, sondern es dauerhaft in der Zeile zu halten.
- **Nötig: jede Admin-Handlung schreibt eine Logzeile mit `player_id`.** Ohne
  sie wäre das Flag genau der Fehler des Kiosks mit einem anderen Namen.

## Offene Punkte

Bewusst nicht entschieden, weil sie die Entscheidung oben nicht berühren:

1. **Wie das Flag weitergegeben wird.** Eine Oberfläche dafür, oder reicht die
   Umgebungsvariable, solange es zwei Leute sind. Neigung: erst die Variable,
   eine Oberfläche wenn es weh tut.
2. **Ob ein Admin ein stärkeres Geheimnis braucht als sechs Ziffern.** Neigung:
   ja, aber nicht jetzt — und das ist die erste Stelle, an der die Schwäche der
   PIN etwas kostet. Wenn Passkeys kommen (#74), löst sich das von selbst.
3. **Ob unumkehrbare Handlungen eine zweite Bestätigung brauchen.** Ein
   gewertetes Ergebnis zu löschen lässt sich nicht zurücknehmen.
4. **Was mit dem Flag geschieht, wenn OIDC kommt.** Gruppenmitgliedschaft
   ersetzt es, oder OIDC weist nur die Identität nach und das Flag bleibt.
   Braucht #89 zuerst.
5. **Kiosk-Freigaben pro Gerät** (#77, Möglichkeit 1): das Token kauft eine
   zufällige, serverseitig gespeicherte Freigabe statt einer abgeleiteten
   Konstante, damit "welche Maschinen sind gerade Kiosk" beantwortbar ist und
   eine einzelne zurückgenommen werden kann. Das lohnt sich unabhängig von
   dieser Entscheidung, braucht aber eine Tabelle und eine kleine
   Admin-Oberfläche — also diese Entscheidung zuerst.
