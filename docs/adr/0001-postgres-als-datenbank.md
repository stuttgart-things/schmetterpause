# ADR-0001: Postgres als Datenbank

- **Status:** accepted
- **Datum:** 2026-08-21
- **Betrifft:** Persistenz, Deployment

## Kontext

Die Anwendung soll in drei Umgebungen laufen: Docker Compose (lokal), Kubernetes
(on-prem) und Azure Container Apps. Persistiert werden Spieler, Matches,
Ergebnisse, TTR-Historie, Ligen und Turniere. Das Datenvolumen ist klein
(Größenordnung: einige Dutzend Spieler, wenige tausend Matches pro Jahr), die
Zugriffsmuster sind relational (Ranglisten, Head-to-Head, Gruppentabellen).

## Betrachtete Optionen

1. Postgres als externer Service
2. SQLite eingebettet ins Binary
3. Kein DBMS, State nur in Redis

## Entscheidung

Wir verwenden Postgres.

## Begründung

- Das Deployment-Ziel entscheidet: In Kubernetes und Azure Container Apps ist der
  Container-Storage ephemer. SQLite bräuchte ein PersistentVolume bzw. einen
  Azure-Files-Mount und bindet die App damit an eine einzige Replica und an einen
  Node. Postgres macht die App zustandslos — das ist die Eigenschaft, die alle
  drei Zielumgebungen gleich behandelbar macht.
- Ranglisten und Gruppentabellen sind Aggregationen über Joins. Window Functions
  (`rank()`, `lag()` für TTR-Verläufe) sparen erheblichen Anwendungscode.
- Managed-Angebote existieren in allen Zielumgebungen (Azure Database for
  PostgreSQL Flexible Server, CloudNativePG in K8s, Container in Compose).

## Konsequenzen

- **Positiv:** App-Container bleibt stateless und horizontal skalierbar. Backups,
  Migrationen und Restore sind gelöste Probleme.
- **Negativ:** Die lokale Entwicklung braucht einen zweiten Container. Ein
  `docker compose up` ist Voraussetzung für den ersten Start — kein
  Single-Binary-Betrieb ohne Abhängigkeiten.
- **Negativ:** Für einen Betrieb auf einem einzelnen Homelab-Host ist Postgres
  Overhead, der fachlich nicht gebraucht wird.

## Verworfene Alternativen

SQLite wäre für die Datenmenge fachlich völlig ausreichend und würde die lokale
Entwicklung vereinfachen (kein Compose nötig, Testdatenbank als Temp-Datei).
Verworfen ausschließlich wegen der Deployment-Anforderung: Die Portierbarkeit
über Compose, K8s und ACA hinweg ist ein explizites Projektziel, und SQLite würde
sie an ein Storage-Backend pro Umgebung koppeln.

Sollte das Projekt dieses Ziel je aufgeben und nur noch auf einem Host laufen,
ist ein Wechsel zu SQLite eine sinnvolle Vereinfachung. Der Datenzugriff wird
deshalb über eine Repository-Schnittstelle gekapselt, damit dieser Weg offen
bleibt.

Redis als alleiniger Store scheidet aus, weil TTR-Historie und Match-Ergebnisse
dauerhaft und konsistent sein müssen. Siehe ADR-0002.
