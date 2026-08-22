# ADR-0005: Kubernetes Custom Resources als Datenspeicher

- **Status:** proposed — Kandidat, nicht entschieden
- **Datum:** 2026-08-22
- **Betrifft:** Persistenz, Deployment
- **Bezug:** verweist auf `0001-postgres-als-datenbank`, ersetzt es nicht

## Kontext

Die Idee: Spieler, Ligen, Turniere und Ranglisten als Custom Resources auf einem
Kubernetes-Cluster ablegen. Der Zustand läge dann in etcd, die Anwendung bräuchte
zur Laufzeit keine eigene Datenbank, und ein Controller würde die Wertung
fortschreiben.

Der Reiz ist echt und nicht nur ästhetisch. Eine Spielerliste, die per GitOps
gepflegt wird, `kubectl get spieler` als Abfrage, Ranglisten im `status` einer
Resource — das integriert die Anwendung in die Plattform, statt sie daneben
zu stellen. Für ein Büro-Werkzeug in einer Umgebung, in der ohnehin ein Cluster
läuft, ist das ein Produktvorteil und kein Selbstzweck.

Dieses ADR hält den Kandidaten fest. Es entscheidet nichts.

## Das übliche Gegenargument trägt hier nicht

„Benutzt etcd nicht als Datenbank" ist bei Massendaten richtig. Für die Mengen
aus ADR-0001 — einige Dutzend Spieler, wenige tausend Matches pro Jahr — trägt
es nicht:

- 3.000 Matches pro Jahr zu je rund 1,5 KB als Custom Resource sind etwa
  **4,5 MB im Jahr**. Die Standard-Quota von etcd liegt bei 2 GiB.
- Ein gewertetes Match schreibt in etwa vier Objekte. Ein Turnierabend mit
  30 Matches sind **120 Schreibvorgänge über drei Stunden**. Für Raft ist das
  Leerlauf.

Wer diesen Kandidaten verwirft, muss es also mit anderen Argumenten tun.

## Was tatsächlich dagegen spricht

### 1. Invariante 1

Ein Binary, ein Image, dieselbe Sache in Docker Compose, Kubernetes und Azure
Container Apps. Ein CRD-Store existiert nur dort, wo ein API-Server steht. Auf
dem Laptop an der Tischtennisplatte (`docs/turnier-vor-ort.md`) gibt es keinen.

Damit wäre ein CRD-Backend nie *der* Speicher, sondern immer ein **zweiter**.
Invariante 5 und ADR-0001 halten diesen Weg ausdrücklich offen — der Datenzugriff
liegt hinter Repository-Schnittstellen, genau damit ein Wechsel möglich bleibt.
Der Preis ist trotzdem real: zwei Implementierungen müssen sich identisch
verhalten, und die Repository-Tests laufen heute gegen echtes Postgres. Für ein
CRD-Backend käme envtest oder kind in der Pipeline dazu.

### 2. „Keine Datenbank" heißt nicht „weniger Teile"

Statt einer Verbindungszeichenfolge braucht der Betrieb CRD-Installation,
ServiceAccount, RBAC und Cluster-Rechte für das Ausrollen. Das ist mehr
Deployment, nicht weniger. Wer den Kandidaten mit „einfacher" begründet,
begründet ihn falsch — er ist *integrierter*, und das ist ein anderes Argument.

### 3. Keine Transaktion über Objekte hinweg

Heute wird ein Match in einem `InTx` gewertet: Match anlegen, beide TTR-Werte
fortschreiben, Historie schreiben — atomar oder gar nicht. Custom Resources
kennen nur optimistisches Sperren pro Objekt über `resourceVersion`.

Die veranstaltungsweise Wertung (siehe CLAUDE.md: Erwartungswerte über alle
Einzel eines Spielers summieren und **einmal** verrechnen) wäre damit ein
Controller mit Reconcile-Schleife, Teilzuständen und Wiederholungen. Das ist
genau das, wofür Controller gebaut sind — aber die Korrektheit wandert aus
Postgres in unseren Code. Bei einer Wertung, die per Definition
reihenfolgeunabhängig sein soll, ist das die teuerste Stelle, an der man sie
haben kann.

### 4. Keine Aggregationen

Die Rangliste ist heute ein Join mit Filter auf `status = 'confirmed'`. Mit
Custom Resources bleiben zwei Wege: alle Matches auflisten und im Speicher
rechnen (bei diesen Mengen vertretbar), oder eine `Standings`-Resource, die ein
Controller pflegt und deren Korrektheit man selbst verantwortet.

## Wo die Idee gut ist

Nicht alle Daten sind gleich. Der Schnitt, der sich anbietet:

| Daten | Als Custom Resource? |
| --- | --- |
| Spieler, Ligen, Turnierdefinitionen | **Ja.** Wenig, selten geändert, deklarativ. `kubectl get spieler` ist ein Feature, und eine Spielerliste per GitOps passt zu einem Büro-Werkzeug. |
| Ranglisten, Tabellen | **Ja, als `status`-Subresource.** Abgeleiteter Zustand gehört im Kubernetes-Modell genau dorthin. |
| Matches, TTR-Historie | **Nein.** Der heiße, fortschreibende, transaktionale Teil — das ist der Grund, warum ein Speicher existiert. |

## Die Vorbedingung

Wenn das Ziel „keine Datenbank zur Laufzeit" ist, dann ist **SQLite** die
naheliegendere Antwort, und ADR-0001 nennt sie bereits: fachlich völlig
ausreichend, verworfen ausschließlich wegen der Portabilität über drei
Umgebungen.

Custom Resources scheitern an derselben Anforderung. Beide Optionen hängen an
einer einzigen Entscheidung:

> **Gilt Invariante 1 weiterhin — ein Image für Compose, Kubernetes und ACA?**

Solange die Antwort ja lautet, bleibt Postgres richtig und ein CRD-Backend wäre
ein Zweitsystem. Fällt sie irgendwann anders aus, öffnen sich beide Türen
gleichzeitig — und dann ist der Schnitt oben der Weg, den ich vorschlagen würde:
Stammdaten als Custom Resources, Matches in einem Speicher, der Transaktionen
kann.

## Nächster Schritt

Keiner. Dieses ADR wird zur Entscheidung, wenn jemand Invariante 1 zur Diskussion
stellt — nicht vorher. Wer es dann aufgreift, sollte zuerst prüfen, ob die
veranstaltungsweise Wertung als Controller-Reconcile ohne Mehrobjekt-Transaktion
überhaupt korrekt formulierbar ist. Das ist die Frage, an der der Kandidat
scheitert oder nicht.
