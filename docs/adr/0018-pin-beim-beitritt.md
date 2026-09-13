# ADR-0018: PIN beim Beitritt, der Code auch als Datei

- **Status:** accepted
- **Datum:** 2026-09-13
- **Betrifft:** Authentifizierung, Oberfläche
- **Bezug:** supersedes `0007-pin-als-anmeldung` im Punkt „Optional" und
  `0006-wiederherstellungscode` im Punkt „nicht als Datei". Alles andere in
  beiden gilt weiter.

## Kontext

Rückmeldung nach dem Update der Büro-Instanz auf den Stand von `main`
(13.09.), mit Screenshots der Anmelde- und der Beitrittskarte:

- **Der Weg für Neue ist schwer zu finden.** Die Anmeldekarte beginnt mit
  einer Namensliste, in der ein Neuer nicht steht; „Neu eintragen" kommt erst
  unter PIN-Feld, Umschalter und Anmelde-Knopf.
- **Die Anmeldekarte zeigt zu viel auf einmal.** Namensauswahl, PIN-Feld und
  ein Link „Ich habe nur den Wiederherstellungscode", bevor überhaupt ein Name
  gewählt ist.
- **Der Beitritt verwirrt.** Zuerst erscheint ein Code, den man aufheben soll,
  darunter „PIN, wenn du magst". Erwartet wird die umgekehrte Reihenfolge: erst
  das Geheimnis wählen, das man im Kopf behält, dann das für den Notfall
  bekommen.
- **Abmelden wird nicht gefunden.** Es steht unten auf dem eigenen Profil,
  unter zwei Karten.

ADR-0007 hat die PIN optional gemacht, damit der Beitritt das
Interaktionsbudget aus AP7 nicht belastet. Der Preis dafür steht schon in
Issue #88: realistisch speichern wenige den Code, also trägt die PIN die
tägliche Last — und eine optionale PIN setzt nur, wer das Angebot unter dem
Code nicht überscrollt. Wer dann das Cookie verliert, hat einen Code, den er
nicht gespeichert hat. Das ist der Weg, auf dem nach ADR-0017 doppelte Spieler
entstehen.

ADR-0006 lehnt eine Datei ab, weil ein Artefakt, das man nur im Schadensfall
braucht, bis dahin meist verloren ist.

## Entscheidung

1. **Die PIN ist beim Beitritt Pflicht.** Sie steht im selben Formular wie der
   Name, mit der Regel und einem Beispiel daneben, und wird geprüft, bevor der
   Spieler entsteht. Über den Beitritt entsteht kein Spieler ohne PIN.
2. **Der Code kommt danach**, weiterhin automatisch erzeugt und einmalig
   angezeigt — und **zusätzlich als Textdatei angeboten**. Die Datei wird aus
   der Seite erzeugt (`data:`-URL mit `download`-Attribut): kein Endpunkt, kein
   Link, nichts, was der Server dafür aufhebt. Dasselbe gilt für einen Code,
   den man sich später im Profil neu ausstellt.
3. **Unverändert:** Spieler, die vor diesem ADR beigetreten sind, und Spieler
   vom Kiosk bleiben ohne PIN, bis sie selbst eine setzen. Der Kiosk setzt
   weiterhin keine PIN für jemanden (ADR-0007, offener Punkt 3). Die
   Ziffern-Regel beim Setzen bleibt.

Mitgeändert, weil es dieselbe Rückmeldung ist und kein ADR berührt:

- „Neu hier?" steht oben auf der Anmeldekarte.
- Das Feld für das Geheimnis erscheint erst, wenn ein Name gewählt ist
  (CSS `:has`, ohne Skript und ohne Request).
- Ein Feld für PIN und Code, ohne Umschalter.
- Abmelden steht oben im eigenen Profil.

## Begründung

### Warum Pflicht

Die PIN ist der Schlüssel, den man im Kopf hat; der Code ist der, den man
verliert. Eine optionale PIN macht den verlierbaren zum einzigen, den die
meisten haben — und genau dann ist Issue #70 zurück, nur einen Schritt später.

Der Preis ist ein Feld mehr beim ersten Besuch, und es ist genau der Preis, den
ADR-0007 vermeiden wollte. Er fällt aber einmal an, im selben Formular und mit
demselben Absenden: ein Feld mehr, kein Schritt mehr. Jeder spätere Scan kostet
nichts zusätzlich. Ein fehlender Weg zurück kostet dagegen jedes Mal, wenn ein
Browser seinen Spieler vergisst.

Vor dem Anlegen geprüft statt danach abgefragt, weil ein Spieler, den es schon
gibt und der nach der PIN gefragt wird, einen geschlossenen Tab davon entfernt
ist, keine zu haben.

### Warum das Beispiel zufällig ist

Ein Beispiel neben einem leeren Feld wird abgeschrieben. Ein festes Beispiel
wäre damit die häufigste PIN im Büro und der erste Rateversuch für jeden
Spieler. Deshalb wird es bei jeder Anzeige neu gezogen.

### Warum jetzt doch eine Datei

Das Argument aus ADR-0006 gilt für die Datei als **einzigen** Ort: ein
Download-Ordner wird aufgeräumt, ein Passwortmanager nicht. Als Angebot
**neben** der Anzeige senkt sie die Schwelle genau im Moment der Anzeige: nicht
jeder hat den Passwortmanager auf dem Handy offen, und der bisherige Rat aus
`docs/turnier-vor-ort.md` — „Screenshot machen" — erzeugt auch nur eine Datei,
eine schlechtere. Der Hinweis auf den Passwortmanager bleibt deshalb stehen.

Die Einwände gegen einen **Link** betrifft das nicht. Es gibt keine URL, die
ein Chat-Vorschau-Roboter aufrufen könnte, und keinen Endpunkt, der einen Code
ein zweites Mal ausgibt: die Datei entsteht aus der Antwort, die den Code
ohnehin einmal im Klartext enthält.

## Konsequenzen

- **Positiv:** Jeder neue Spieler hat zwei Wege zurück statt meistens einem.
  Die Beitrittskarte zeigt danach nur noch eine Sache, den Code.
- **Einschränkung:** Der Beitritt ist ein Feld länger. Wer die Regel verfehlt,
  tippt nach der Ablehnung die PIN neu — sie wird wie überall nicht in die
  Seite zurückgeschrieben.
- **Einschränkung:** Beim Anmelden öffnet das Handy nicht mehr den
  Ziffernblock (Issue #117), weil ein Feld für beides eine Tastatur braucht,
  die Buchstaben kann. Ziffern sind dort einen Tipp entfernt.
- **Einschränkung:** Eingebaute Browser mancher QR-Scanner ignorieren
  `download`. Dort bleibt die Anzeige, wie bisher.
- **Nötig:** Bestehende Spieler ohne PIN bekommen keine erzwungene. Das Profil
  bietet sie an, und der Abmelden-Knopf sagt, was ohne PIN der Preis ist.
