# ADR-0014: Der Kiosk benennt, wer einträgt

- **Status:** accepted
- **Datum:** 2026-09-04
- **Betrifft:** Kiosk, Datenmodell, Betrieb
- **Bezug:** schreibt `0008-wer-fuer-andere-handeln-darf` fort und ersetzt es
  nicht — der Kiosk bleibt „Vertrauen aus dem Raum", er sagt jetzt nur, wer im
  Raum steht. Beantwortet Issue #90.

## Kontext

Ein Kiosk-Ergebnis zählt sofort. Niemand bestätigt es, und damit fällt genau
die eine technische Prüfung weg, die diese Anwendung sonst hat. ADR-0008
begründet das, und die Begründung ist ein Ort: jemand steht am Tisch, die
Leute kennen sich, alle sehen zu.

**Der Ort hört auf zu tragen, sobald die Anwendung auf einem Server läuft.**
Mit einer festen Adresse (#74, #78) ist `/kiosk?token=…` von jedem Telefon im
Netz erreichbar, und die Bedingung schrumpft von „du stehst an der Platte" auf
„du hast das Token irgendwann heute mal gesehen".

Drei Dinge wurden vorher schon gebaut und reichen einzeln nicht:

- **#71** (`matches.entered_via`) macht eine Kiosk-Zeile auffindbar, verhindert
  aber nichts.
- **#77/#103** gibt jedem Gerät eine eigene, widerrufbare Freigabe. Damit ist
  „welche Maschinen sind gerade Kiosk" beantwortbar — welcher *Mensch* daran
  sitzt, nicht.
- **#91** lehnt ein Match ab, an dem der in diesem Browser angemeldete Spieler
  beteiligt ist. Ein privates Fenster mit nur dem Kiosk-Cookie geht daran
  vorbei, und das war von Anfang an als Bremsschwelle beschrieben.

Der gemeinsame Rest: **der Kiosk hat keine Identität.** ADR-0008 sagt das
selbst — „‚Der Kiosk' benennt einen Browser".

Dazu kam eine Nebenwirkung, die in den Daten steht. Ein Kiosk-Match wurde mit
`reported_by = home_id` geschrieben. In der Datenbank sah ein Abend, den eine
Person eingetippt hat, damit aus wie zehn Leute, die ihre eigenen Spiele
melden — der Grund, warum die Definition of Done in #43 eine eigene Spalte
brauchte, um Kiosk-Zeilen überhaupt ausschließen zu können.

## Entscheidung

**Eine freigeschaltete Maschine fragt: wer trägt ein? — und schreibt nichts,
bevor sie eine Antwort hat.**

- `kiosk_grants.operator_id` benennt den Spieler, der gerade tippt. Nullable,
  weil Freischalten und Benennen zwei Schritte sind.
- Jeder schreibende Kiosk-Weg — Ergebnis, Undo, Spieler anlegen, Code
  ausstellen — verlangt einen benannten Operator. Freigeschaltet, aber
  unbenannt zählt wie gesperrt.
- **`matches.reported_by` ist der Operator**, nicht mehr der Heimspieler.
- **Der Operator darf nicht mitspielen.** Serverseitig geprüft, im Turnier wie
  im Alltag. Das ist die Prüfung, die #91 nur annähern konnte: sie hängt am
  Grant und nicht am Cookie des Browsers, also läuft ein privates Fenster in
  dieselbe Ablehnung.
- **Der Stift wird weitergereicht**, nicht die Freigabe: ein zweiter Name
  ersetzt den ersten, ohne dass die Maschine neu freigeschaltet wird. Wer
  gerade einträgt, steht auf der Kiosk-Seite und unter `/admin`.
- Jede Kiosk-Zeile im Log trägt die `operator_id`.

**Das ist keine Anmeldung.** Der Laptop bekommt keine Sitzung, und das ist
Absicht: acht Leute teilen ihn sich, und er darf nicht als einer von ihnen
angemeldet enden — dieselbe Regel, die schon dafür sorgt, dass „Spieler
anlegen" am Kiosk den Browser nicht anmeldet.

**Und es ist kein Beweis.** Wer das Token hat, kann einen fremden Namen
wählen. Der Raum trägt weiterhin — diese Entscheidung macht nur lesbar, was er
getragen hat.

## Warum nicht anders

**Admin-Sitzung zum Freischalten.** Die härteste Variante: Token allein reicht
nicht, die Maschine muss zusätzlich als Admin angemeldet sein. Sie schließt
die Lücke wirklich, statt sie nur zu benennen. Abgelehnt wegen des Preises am
Abend: `SP_BOOTSTRAP_ADMIN` würde zur Pflicht, der Admin müsste vor dem
Aufsetzen beitreten und `office:up` ein zweites Mal laufen — und der geteilte
Laptop wäre als eine bestimmte Person angemeldet, also genau das, was der
Kiosk nicht sein soll. Bleibt die richtige Antwort, falls sich zeigt, dass ein
Name zu wenig ist.

**Kiosk-Ergebnisse warten auf Bestätigung.** Braucht gar keine Identität, weil
der Gegner wieder die Prüfung ist. Abgelehnt, weil es dem Kiosk genau die
Eigenschaft nimmt, für die es ihn gibt: eine Person schreibt einen Abend mit,
und niemand hat ein Handy in der Hand, um zu bestätigen. Für das Turnier wäre
es eine Warteschlange statt einer Tabelle.

**Die Ergebniseingabe am Kiosk abschaffen.** Ernst zu nehmen: die Messwoche
(#43) hatte **null Kiosk-Einträge bei 45 Matches**, und seit ADR-0010 tippen
Spieler ihre Turnierergebnisse selbst vom Handy. Trotzdem abgelehnt — der
Fall, den der Kiosk beantwortet, ist „ich hab mein Handy nicht dabei", und der
verschwindet nicht dadurch, dass er in einer Woche nicht vorkam. Dieselbe
Woche senkt allerdings den Preis dieser Entscheidung: es gibt keine
Gewohnheit, die sie bricht.

## Konsequenzen

- Der Turnierabend hat einen Schritt mehr: einmal auswählen, wer tippt.
  `docs/turnier-vor-ort.md` sagt es an der Stelle, an der es passiert.
- **`reported_by` sagt bei Kiosk-Zeilen ab jetzt etwas anderes als vorher.**
  Alte Zeilen nennen weiterhin den Heimspieler. Die Definition of Done in
  `scripts/definition-of-done.sql` filtert über `entered_via` und
  `tournament_id` und ist davon nicht betroffen; wer künftig über
  `reported_by` auswertet, muss den Bruch kennen.
- Wer eintippt, taucht in der Rangliste auf wie jeder andere — er spielt ja
  auch. Seine Wertung bewegt sich durch das Eintragen nicht.
- Ein Spieler, der entfernt wird, nimmt die Freigabe nicht mit
  (`on delete set null`); die Maschine fragt dann wieder.
