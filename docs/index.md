# Schmetterpause

Matchmaking-, Liga- und Turnier-App für Büro-Tischtennis. Ergebnisse erfassen,
vom Gegner bestätigen lassen, TTR berechnen, Rangliste anzeigen — und irgendwann
Ligen, Turniere und Buchungen an der Platte.

Diese Seiten sind die Dokumentation zum Projekt. Der Code liegt auf
[GitHub](https://github.com/stuttgart-things/schmetterpause).

## Wo anfangen

Je nachdem, warum jemand hier gelandet ist:

- **Ein Abend an der Platte steht an.** [Turnier vor Ort](turnier-vor-ort.md)
  ist die Anleitung dafür: ein Laptop für alle, ein QR-Code an der Wand, alle
  tragen vom eigenen Handy ein. Lesbar auch im Stehen, während die ersten
  schon aufschlagen.
- **Was ist eigentlich geplant?** Der [MVP-Plan](mvp-plan.md) hält Scope,
  Datenmodell und Arbeitspakete fest, samt der Definition of Done, an der der
  Meilenstein gemessen wird.
- **Warum ist das so gebaut?** Die [Entscheidungen](adr/index.md) — ein ADR je
  Festlegung, jeweils mit den Alternativen, die dabei verworfen wurden.
- **Das Ding soll irgendwo laufen.** [Deployment](deployment.md) beschreibt den
  Weg auf einen Cluster ohne GitOps dazwischen. Englisch, wie die übrige
  Betriebsdokumentation: sie liest, wer den Cluster bedient.

## Ein paar Begriffe

**TTR** ist das Tischtennis-Rating nach dem deutschen Verbandssystem. Es
funktioniert wie eine Elo-Zahl, rechnet aber mit dem Divisor 150 statt 400 — ein
Unterschied von 150 Punkten entspricht also einer Siegwahrscheinlichkeit von
90 %, nicht erst einer von 300 Punkten. Gewertet werden nur Einzel.

**Veranstaltungsweise Wertung** heißt: bei Turnieren und Ligarunden werden die
Erwartungswerte über alle Einzel eines Spielers summiert und einmal verrechnet,
nicht nach jedem Match einzeln. Damit hängt das Ergebnis nicht davon ab, in
welcher Reihenfolge jemand die Zettel eintippt.

**Slot** ist ein buchbares Zeitfenster an einer Platte. Turnierspiele belegen
Slots über denselben Mechanismus wie eine normale Buchung — das Turniermodell
sitzt darüber, nicht daneben.

## Zum Namen

Schmetterball plus Pause. Entstanden aus einer Abstimmung im Büro und gesetzt:
der Name steckt im Modulpfad, im Image-Namen und im Browser-Titel.
