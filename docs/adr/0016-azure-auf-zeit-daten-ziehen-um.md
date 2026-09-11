# ADR-0016: Azure läuft auf Zeit, die Spieldaten ziehen um

- **Status:** accepted
- **Datum:** 2026-09-10
- **Betrifft:** Deployment, Betrieb, Daten
- **Bezug:** schreibt `0001-postgres-als-datenbank` fort und stützt sich auf
  `0007-pin-als-anmeldung`. Ersetzt kein ADR. Terraform in #206, die
  Umsetzung dieser Entscheidung in #213, Crossplane später in #214.

## Kontext

Seit v0.6.0 läuft Schmetterpause auch auf Azure Container Apps, gebaut aus
`terraform/`. Damit gibt es drei Orte, an denen die Anwendung eine Datenbank
hat:

| Umgebung | Wofür | Postgres heute |
| --- | --- | --- |
| Compose | Entwicklung, lokales Testen | `postgres:18-alpine` |
| Kubernetes | das Büro, Preview-Umgebungen | CloudNativePG, `postgresql:17` |
| Azure | Tests und einzelne Anlässe | Flexible Server, `17` |

Azure kostet, solange es steht — der Flexible Server auch dann, wenn niemand
spielt. Für eine Tischtennis-App im Büro gibt es keinen Grund, das dauerhaft zu
bezahlen.

Gleichzeitig sind die Daten nicht mehr wegwerfbar (#7, #194): echte Matches,
echte Wertungen, Spieler mit PIN. Wer Azure abbaut und später wieder aufbaut,
will dieselbe Rangliste vorfinden. Und je nach Anlass soll mit denselben Daten
mal auf Kubernetes, mal auf Azure gespielt werden.

`terraform destroy` löscht den Flexible Server samt seiner eigenen Backups.
Ohne einen Schritt davor ist jeder Abbau ein Datenverlust.

## Entscheidung

**Azure wird nur auf Zeit betrieben. Die Spieldaten ziehen als logischer Dump
zwischen den Umgebungen um, und es schreibt immer genau eine.**

1. **Azure ist nie der Dauerzustand.** Angelegt für einen Test oder einen
   Anlass, danach `task tf:destroy`. Was dort vereinfacht ist — die
   Firewall-Regel für alle Azure-Dienste, der lokale Terraform-State — ist
   unter dieser Annahme vertretbar und wäre es im Dauerbetrieb nicht.
2. **Kein Abbau ohne Dump, kein Neuanlegen ohne Restore.** Beides gehört zum
   Ablauf selbst, nicht zu einer Checkliste daneben.
3. **Ein Format für alle drei Umgebungen:** ein logischer `pg_dump` mit Schema
   und Daten, `goose_db_version` eingeschlossen, eingespielt mit
   `--no-owner --no-acl` in eine **leere** Datenbank. Die Anwendung migriert
   danach vorwärts, wie bei jedem Start.
4. **Eine Postgres-Hauptversion überall.** Heute nicht erfüllt, siehe Tabelle;
   welche es wird, entscheidet #213. Bis dahin gilt: ein Dump geht nur in
   dieselbe oder eine neuere Hauptversion, nie zurück. Für die Anwendung
   genauso — das Ziel läuft mit derselben oder einer neueren Version als die
   Quelle.
5. **Es schreibt immer nur eine Umgebung.** Daten werden nicht
   zusammengeführt, nicht synchronisiert, nicht repliziert. Ein Umzug heißt:
   Dump der aktiven Umgebung, Restore in die nächste — und die alte ist ab da
   keine Quelle mehr. Zwei Umgebungen, die gleichzeitig Ergebnisse annehmen,
   gibt es nicht.
6. **Compose empfängt, sendet aber nie.** Ein Dump aus Kubernetes oder Azure
   auf dem Laptop ist der beste Testdatensatz, den es gibt. Der umgekehrte Weg
   ist ausgeschlossen: was auf einem Laptop entsteht, geht nicht in eine
   Umgebung, in der gespielt wird.

## Was daraus folgt

**Nach einem Umzug meldet sich jeder einmal neu an.** Das
Wiedererkennungs-Cookie trägt kein `Domain` und hängt damit am Host
(`internal/auth/cookie.go`). Eine andere Umgebung ist ein anderer Host — auch
ein neu angelegtes Azure-Environment, das eine neue generierte Domain bekommt.
Tragbar ist das, weil der Dump die PIN-Hashes und Wiederherstellungscodes
mitnimmt (ADR-0006, ADR-0007): wer sich anmeldet, ist wieder er selbst, mit
seiner Wertung. Wer weder PIN noch Code hat, steht nach einem Umzug vor
derselben Lage wie nach einem verlorenen Gerät. `SP_SESSION_KEY` muss nur dann
mitziehen, wenn die Adresse gleich bleibt, also hinter einer eigenen Domain.

**Ein Dump ist ein Geheimnis.** Er enthält Anzeigenamen, PIN-Hashes und
Code-Hashes. Er gehört nicht ins Repository (`/schmetterpause-*.sql` ist schon
ignoriert) und nicht als Artefakt in eine Pipeline. Wo er zwischen Abbau und
Neuanlegen liegt, entscheidet eine Person.

**Plattform-Backups bleiben, was sie sind.** Velero oder barman-cloud (#84) und
die Backups des Flexible Server schützen *eine* Umgebung gegen ihren eigenen
Ausfall. Umziehen können sie nicht: ein barman-Backup lässt sich nicht in einen
Flexible Server einspielen, ein Azure-Backup nicht in CloudNativePG. #84 wird
durch diese Entscheidung nicht ersetzt, sondern abgegrenzt — dort
Wiederherstellung am selben Ort, hier Umzug.

**Crossplane später** (#214) ändert, *wie* Azure angelegt und abgebaut wird,
nicht *dass* es abgebaut wird. Ein gelöschter Claim ist ein `destroy`, und
Punkt 2 muss dort eingebaut sein, statt erinnert zu werden.

## Warum nicht anders

**Azure dauerhaft laufen lassen.** Kostet jeden Monat für eine App, die an
einem Bürotisch hängt, und wäre die zweite dauerhafte Umgebung, an der #194 die
Kargo-Frage (#83) wieder aufmacht. Nichts, was gerade gebraucht wird,
rechtfertigt das.

**Nur die App stoppen, den Server behalten.** Ein Flexible Server lässt sich
höchstens sieben Tage anhalten und startet danach von selbst wieder, und der
Speicher kostet auch im Stillstand. Das verschiebt den Abbau, es ersetzt ihn
nicht.

**Die Plattform-Backups als Umzugsformat.** Jedes ist an seine Plattform
gebunden, siehe oben. Ein logischer Dump ist das einzige Format, das Compose,
CloudNativePG und Flexible Server gleichermaßen lesen.

**Nur die Daten dumpen, ohne Schema.** Bindet den Dump an genau die
Schemaversion des Ziels und scheitert, sobald Quelle und Ziel eine Migration
auseinanderliegen. Schema und `goose_db_version` mitzunehmen lässt die
Anwendung am Ziel vorwärts migrieren, so wie bei jedem Deployment.

**Synchronisation zwischen Umgebungen** — logische Replikation oder ein
Abgleich durch die Anwendung. Löst ein Problem, das es nicht gibt: es entstehen
nie gleichzeitig Daten an zwei Orten. Und es verlangte, dass sich die
Umgebungen gegenseitig erreichen, über Netze, die dafür nicht gebaut sind.
