# ADR-0002: Kein Redis, bis ein konkreter Auslöser eintritt

- **Status:** accepted
- **Datum:** 2026-08-21
- **Betrifft:** Architektur, Deployment

## Kontext

Die ursprüngliche Architekturskizze sah Redis als festen Bestandteil vor, mit dem
Verwendungszweck "Caching". Bei der Durchsicht ließ sich dieser Zweck nicht
belegen: Bei einer Nutzerzahl im niedrigen zweistelligen Bereich beantwortet
Postgres alle Lese-Abfragen (Rangliste, Tabelle, Historie) im
Sub-Millisekunden-Bereich. Ein Cache würde Invalidierungslogik einführen, ohne
ein bestehendes Problem zu lösen.

Es gibt jedoch zwei Anforderungen, für die Redis die naheliegende Lösung ist —
beide sind im MVP nicht enthalten.

## Entscheidung

Redis wird nicht aufgenommen. Es wird aufgenommen, sobald einer der folgenden
Auslöser eintritt:

1. Die App läuft mit mehr als einer Replica und nutzt SSE. Live-Score-Events
   müssen dann an Verbindungen auf allen Replicas verteilt werden. Ohne Redis
   Pub/Sub (oder Postgres `LISTEN/NOTIFY`) sieht ein Client nur die Events, die
   auf seiner eigenen Replica entstehen.
2. Die Platten-Buchung wird umgesetzt. Zwischen "Slot ausgewählt" und "Buchung
   committed" braucht es einen kurzlebigen Lock, um Doppelbuchungen zu
   verhindern. `SET NX` mit TTL ist dafür das passende Primitiv.

Bis dahin gilt: In-Process-Caching, wo überhaupt nötig.

## Konsequenzen

- **Positiv:** Zwei Container statt drei in allen Zielumgebungen. Einfacheres
  Compose-File, ein Backing Service weniger in K8s und ACA.
- **Positiv:** Keine Cache-Invalidierung, damit eine ganze Klasse von
  Inkonsistenz-Bugs ausgeschlossen.
- **Negativ:** Wenn Auslöser 1 eintritt, ist Redis nachträglich einzuziehen. Der
  Aufwand wird auf wenige Stunden geschätzt, da die Event-Verteilung ohnehin
  hinter einer Schnittstelle liegt.
- **Risiko:** Diese Entscheidung sieht ohne Kontext wie ein Versäumnis aus. Genau
  deshalb existiert dieses ADR.

## Hinweis für später

Falls nur Auslöser 1 eintritt und Auslöser 2 dauerhaft ausbleibt, ist Postgres
`LISTEN/NOTIFY` die schlankere Alternative zu Redis Pub/Sub — es kommt ohne
zusätzlichen Service aus. Für den Lock-Anwendungsfall taugt es nicht, dafür
braucht es Redis oder `SELECT ... FOR UPDATE SKIP LOCKED`.
