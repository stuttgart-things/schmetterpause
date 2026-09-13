# ADR-0019: Backup auf Kubernetes mit dem Barman-Cloud-Plugin, Umzug erst nach gemessenem Restore

- **Status:** accepted
- **Datum:** 2026-09-13
- **Betrifft:** Betrieb, Daten, Deployment
- **Bezug:** supersedes `0016-azure-auf-zeit-daten-ziehen-um` in Punkt 6
  („Compose empfängt, sendet aber nie") für genau einen Umzug; alles andere in
  ADR-0016 gilt weiter. Setzt #84 um und ersetzt dessen Entscheidungsregel.
  Plattform: stuttgart-things/stuttgart-things#2799.

## Kontext

Das Büro spielt nicht dort, wo ADR-0016 es einträgt. Die Tabelle dort nennt
Kubernetes „das Büro"; tatsächlich liegen die echten Ergebnisse seit Wochen in
der lokalen Compose-Instanz — am 13.09. 16 Spieler und 65 bestätigte Matches —,
gesichert durch Dumps von Hand. Die CloudNativePG-Instanz auf homerun2-test1 hat
6 Spieler und wurde nie wirklich bespielt.

Am 13.09. entschieden: **ab jetzt Kubernetes — aber erst, wenn dort Backup und
Restore stehen.**

#84 hatte zwei Wege vorgesehen und eine Regel festgelegt: Velero mit Dump-Hook
(Weg A) oder CloudNativePG mit barman-cloud (Weg B), „A, außer A fällt bei einem
Kriterium durch, das zählt". Seitdem gemessen und nachgelesen:

| | |
| --- | --- |
| Velero auf homerun2-test1 | nicht installiert |
| Volume-Snapshots | keine VolumeSnapshotClass, nur hostpath-Provisioner |
| CloudNativePG | 1.30.0 — `spec.backup.barmanObjectStore` abgekündigt, **in 1.31.0 entfernt** |
| Barman-Cloud-Plugin | im Org schon in Betrieb (flux, labda-dev-a), Restore dort in 51–82 s |
| Object Store | MinIO auf platform-sthings; `cluster-trust-bundle` vertraut dessen Zertifikat (geprüft) |

## Entscheidung

1. **Die Spieldatenbank auf Kubernetes wird über das Barman-Cloud-Plugin
   gesichert:** kontinuierliche WAL-Archivierung und ein täglicher Base-Backup
   in einen eigenen Bucket. Kein Velero auf homerun2-test1, kein eingebautes
   `barmanObjectStore`.
2. **Gerendert im argocd-Katalog** — `apps/schmetterpause/database`, Schalter
   `database.backup`, standardmäßig aus — und im Consumer für homerun2-test1
   eingeschaltet. Nicht in `kcl/database.k`: der Weg ohne Argo bekommt kein
   Backup-Ziel, weil ein Object Store Umgebungskonfiguration ist.
3. **Die Büro-Daten ziehen erst um, wenn drei Dinge gezeigt sind:**
   - `ContinuousArchiving=True` am Cluster,
   - mindestens ein geplanter Backup in Phase `completed`,
   - ein **zeitgemessener Restore in einen leeren Namespace**, nach
     schriftlicher Anleitung, mit denselben Zählwerten wie das Original.
4. **Der Umzug selbst ist der Weg aus ADR-0016:** `task db:dump ENV=compose`,
   `task db:restore ENV=kubernetes`, direkt danach ein manueller Backup. Dafür
   sendet Compose **dieses eine Mal** — die Ausnahme von Punkt 6, und nur diese.
   Ab dem Restore ist Kubernetes der einzige Schreiber (Punkt 5); Compose
   empfängt danach wieder nur Dumps zum Testen.
5. **Aufbewahrung 30 Tage.** Wiederherstellung auf einen beliebigen Zeitpunkt
   fällt mit der WAL-Archivierung ab, ist aber kein Kriterium (#84).

## Begründung

### Warum nicht Velero

Die Regel aus #84 greift, und zwar beim Kriterium „was das Backup enthält".
Ohne Snapshots und ohne Node-Agent sichert Velero auf diesem Cluster nur
Kubernetes-Objekte — der Dump, den ein Hook auf das Volume schreibt, wäre nicht
dabei. Mit Node-Agent wäre es ein clusterweites DaemonSet für eine Datenbank
von Kilobytes, auf einem Cluster, auf dem Velero heute gar nicht läuft.

### Warum das Plugin und nicht das eingebaute barmanObjectStore

Ein Backup, das mit dem nächsten Operator-Update still aufhört möglich zu sein,
ist das falsche. 1.31 ist angekündigt, und Renovate bewegt die Operator-Charts.

### Warum der Restore vor dem Umzug

Ein Backup, das nie zurückgespielt wurde, ist keins. Vor dem Umzug kostet der
Test nichts: die echten Daten liegen noch auf Compose, und ein Fehlschlag trifft
eine Instanz mit sechs Testspielern.

### Warum Compose hier doch sendet

Punkt 6 in ADR-0016 schließt aus, dass etwas von einem Laptop in eine Umgebung
geht, in der gespielt wird — weil dort sonst Testdaten landen. Hier ist es
umgekehrt: auf dem Laptop liegt das Einzige, was gespielt wurde. Den Dump nicht
zu nehmen hieße, das Büro von vorn anfangen zu lassen. Die Ausnahme gilt diesem
Umzug und hebt Punkt 6 nicht auf.

## Konsequenzen

- **Positiv:** Das Verlustfenster schrumpft von „seit dem letzten Dump von Hand"
  auf die Minuten bis zum nächsten WAL-Segment. Ein Restore ist ein neuer
  Cluster aus dem Object Store, in jeden Namespace.
- **Einschränkung: Bucket und Cluster stehen im selben Lab.** Fällt das Lab,
  fällt beides. Eine Kopie außerhalb ist nicht Teil dieser Entscheidung.
- **Einschränkung: `SP_SESSION_KEY` ist nicht im Backup.** Er liegt in Vault
  (Eintrag `schmetterpause`). Ein Restore ohne ihn meldet alle ab — PINs und
  Wiederherstellungscodes kommen aber mit der Datenbank zurück (ADR-0007), es
  ist also ein Neu-Anmelden und kein Datenverlust. Nach dem Umzug meldet sich
  ohnehin jeder einmal neu an, weil der Host ein anderer ist (ADR-0016).
- **Einschränkung:** Das Einschalten startet den Postgres-Pod neu — also
  außerhalb der Spielzeit.
- **Nötig von der Plattform** (#2799): Bucket und eigener MinIO-User, ein
  eigener Vault-Eintrag `schmetterpause-backup` — nicht der Eintrag der App, denn
  ein Schreiben des ganzen Eintrags könnte `session-key` zurücksetzen — und das
  Plugin im Namespace des Operators.
- **Nötig:** ein Signal, wenn ein Backup ausbleibt. Bis #175 Stufe 3 die
  CloudNativePG-Metriken abgreift, sieht das niemand von selbst.

## Offene Punkte

1. **Wer den Restore-Test durchführt.** #84 wünscht jemanden, der es nicht
   gebaut hat.
2. **Alarm bei ausbleibendem Backup** — hängt an #175 Stufe 3.
3. **Eine Kopie außerhalb des Labs.**
