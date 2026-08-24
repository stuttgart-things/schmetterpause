# MVP-Plan — Schmetterpause

- **Stand:** 2026-08-21
- **Zuschnitt:** Variante A — "Ergebnisse landen in der App"
- **Voraussetzung:** ADR-0001 bis ADR-0004, `CLAUDE.md`

## Zielsetzung

Der MVP beantwortet genau eine Frage: **Tragen Leute freiwillig Ergebnisse ein?**

Alles, was diese Frage nicht beantwortet, gehört nicht in den MVP. Die
Erfolgsmessung ist verhaltensbasiert, nicht featurebasiert.

### Definition of Done

Der MVP gilt als erfolgreich, wenn über **fünf aufeinanderfolgende Arbeitstage**
mindestens **zehn Matches** von mindestens **fünf verschiedenen Spielern**
eingetragen wurden, **ohne** dass jemand daran erinnert wurde.

Wird das verfehlt, ist der nächste Schritt nicht "mehr Features", sondern die
Ursachenanalyse: zu umständliche Eingabe, fehlender Anlass zu spielen, oder
schlicht kein Interesse. Der Ligamodus (M2) ist die Antwort auf "fehlender
Anlass" — aber nur, wenn die Messung das zeigt.

Abgelesen wird das mit `task office:dod`. Die Anwendung hat dafür keinen
Bildschirm und bekommt auch keinen: es ist eine Zahl, die man einmal liest.

## Scope

### Enthalten

| Bereich | Umfang |
|---|---|
| Spieler | Anlegen mit Anzeigename, Wiedererkennung über signiertes Cookie |
| Match | Erfassen: zwei Spieler, Satzergebnisse, Matchmodus (Best-of-N, Punkte zum Sieg) |
| Bestätigung | Gegner bestätigt oder widerspricht dem eingetragenen Ergebnis |
| TTR | Berechnung nach Verbandsformel, Historie pro Spieler |
| Rangliste | Sortiert nach TTR, mit Anzahl Spiele und Bilanz |
| Match-Historie | Liste aller Matches mit allen Sätzen und der Wertung, neueste zuerst |
| QR-Eingabe | QR-Code an der Platte öffnet direkt die Ergebniseingabe |
| Betrieb | Docker Compose lauffähig, CI mit Lint/Build/Verify |

### Bewusst nicht enthalten

Liga, Turnier, Gruppen/KO, Platten-Buchung, Slots, Redis, SSE/Livescore,
Wall-Display, OIDC/Codehub-Login, Passkeys, Doppel, Handicap, Webhooks,
Kubernetes- und ACA-Deployment.

Kubernetes und ACA sind **nicht** im MVP-Scope, aber die Invarianten aus
`CLAUDE.md` (ein Image, Env-Konfiguration, stateless) gelten trotzdem ab der
ersten Zeile. Der Unterschied: Wir bauen die Manifeste noch nicht, verbauen uns
den Weg dorthin aber auch nicht.

## Datenmodell (MVP-Umfang)

```
players
  id            uuid pk
  display_name  text not null
  ttr           int not null default 1000
  created_at    timestamptz

identities                      -- vgl. ADR-0003, im MVP nur provider='local'
  provider      text
  subject       text
  player_id     uuid fk
  created_at    timestamptz
  pk (provider, subject)

matches
  id            uuid pk
  home_id       uuid fk players
  away_id       uuid fk players
  best_of       int not null            -- 3, 5, 7
  points_to_win int not null default 11
  status        text not null           -- pending | confirmed | disputed
  reported_by   uuid fk players
  played_at     timestamptz
  confirmed_at  timestamptz

match_sets
  match_id      uuid fk matches
  set_no        int
  home_points   int
  away_points   int
  pk (match_id, set_no)

ttr_history
  id            uuid pk
  player_id     uuid fk players
  match_id      uuid fk matches
  ttr_before    int
  ttr_after     int
  created_at    timestamptz
```

`ttr_history` ist im MVP nicht optional. Ohne sie lässt sich eine falsche
Berechnung später nicht nachvollziehen oder korrigieren, und Verlaufsgrafiken
wären nicht nachträglich rekonstruierbar.

## Arbeitspakete

### AP1 — Gerüst

Go-Modul, Verzeichnisstruktur, templ- und HTMX-Setup, Postgres-Migrations
(goose), Repository-Interfaces, `/healthz` und `/readyz`, Dockerfile
(Multi-Stage, distroless), `compose.yaml` mit App und Postgres, Taskfile,
Dagger-Pipeline.

*Fertig, wenn:* `task up` startet die Anwendung, `task ci` läuft lokal grün
durch und liefert dasselbe Ergebnis wie in der Pipeline.

### AP2 — Spieler und Session

Anlage mit Anzeigename, signiertes Cookie, `identities`-Eintrag mit
`provider='local'`, Spielerliste. Kein Passwort.

*Fertig, wenn:* Ein Browser wird über Neustarts hinweg demselben Spieler
zugeordnet.

### AP3 — TTR-Package

Eigenes Package ohne DB- und HTTP-Abhängigkeit. Siegwahrscheinlichkeit,
Änderungskonstante, veranstaltungsweise Wertung (Summe der Erwartungswerte über
mehrere Einzel), kaufmännische Rundung.

*Fertig, wenn:* Tests gegen mindestens fünf von Hand nachgerechnete Fälle
grün sind, inklusive eines Falls mit mehreren Matches in einer Wertung.

**Dieses Paket zuerst bauen.** Es ist reine Fachlogik, komplett testbar und
unabhängig vom Rest — der ideale Kandidat für eine saubere erste Iteration.

### AP4 — Matcherfassung

Formular: Gegner wählen, Matchmodus wählen, Sätze eintragen. Validierung
(Satzergebnisse müssen zum Modus passen, Zwei-Punkte-Abstand, Anzahl
Gewinnsätze). Status `pending`.

*Fertig, wenn:* Ein unplausibles Ergebnis abgelehnt wird und die Fehlermeldung
sagt, warum.

### AP5 — Bestätigung

Der Gegner sieht offene Ergebnisse und bestätigt oder widerspricht. Erst bei
`confirmed` wird TTR gerechnet und `ttr_history` geschrieben. `disputed`
blockiert die Wertung; wer widerspricht, bekommt direkt das Eingabeformular
mit dem gemeldeten Ergebnis vorbefüllt und trägt ein, wie es wirklich
ausgegangen ist — das Match geht damit als `pending` an den anderen zurück.
Korrigieren darf jeder der beiden.

*Fertig, wenn:* Ein unbestätigtes Match die Rangliste nicht beeinflusst.

**Kiosk.** Für einen Turnierabend, an dem ein Rechner an der Platte steht,
gibt es `/kiosk`: dort legt eine Person Spieler an und trägt Ergebnisse
zwischen beliebigen zwei Spielern ein, die **sofort** gewertet werden. Wer
zugesehen und mitgeschrieben hat, ist die Bestätigung — es gibt niemanden
mehr zu fragen. Der Kiosk existiert nur, wenn `SP_KIOSK_TOKEN` gesetzt ist.

Was dort entsteht, ist **nicht** die Messung der Definition of Done: die
fragt, ob Leute *freiwillig* eintragen, und ein Schriftführer am Turnierabend
ist das Gegenteil davon. Kiosk-Spieler haben außerdem keine Identität und
können sich später nicht vom eigenen Handy anmelden.

### AP6 — Rangliste und Spielerprofil

Rangliste mit TTR, Spielen, Bilanz. Profilseite mit letzten Matches und
TTR-Verlauf.

### AP7 — QR-Eingabe

Druckbarer Aushang unter `/qr`, dessen QR-Code direkt in die Ergebniserfassung
springt.

*Fertig, wenn:* Vom Scannen bis zum abgeschickten Ergebnis sind es höchstens
drei Interaktionen.

**Es gibt nur eine Platte.** Damit entfällt der ursprünglich vorgesehene
Code *pro* Platte samt Vorbelegung — es gibt nichts vorzubelegen, keine
Kennung in der URL und keine `tables`-Tabelle, die das Slot-Modell eines
späteren Meilensteins vorwegnehmen würde. Kommt eine zweite Platte dazu, ist
das eine eigene Entscheidung mit eigenem Schemaschritt.

**Der Code wird im Binary erzeugt**, nicht als Bilddatei mitgeliefert. Ein
QR-Code enthält eine absolute URL; eine mitgelieferte Datei müsste einen Host
einbacken und würde Invariante 2 verletzen. Die Adresse stammt deshalb aus dem
Request — `SP_PUBLIC_BASE_URL` überschreibt sie dort, wo ein Proxy davorsteht
und der Request die öffentliche Adresse nicht mehr kennt.

**Das Ziel ist der Anker `#match` auf der Startseite**, nicht eine zweite
Seite mit demselben Formular. Wer noch nicht erkannt ist, landet oben bei der
Namenseingabe — genau die richtige Reihenfolge beim ersten Scan.

### Reihenfolge

AP1 → AP3 → AP2 → AP4 → AP5 → AP6 → AP7

AP3 vor AP2, weil die Fachlogik unabhängig ist und einen frühen, gut testbaren
Erfolg liefert. AP7 zuletzt, weil es die anderen Pakete voraussetzt — aber es
darf **nicht** entfallen: Ohne QR-Code ist die Eingabehürde der wahrscheinlichste
Grund, an der Definition of Done zu scheitern.

## Build und CI/CD

### Aufgabenteilung

**Task (`Taskfile.yml`)** ist die Einstiegsschicht für Menschen. Jeder Befehl,
den ein Entwickler tippt, ist ein Task. Tasks enthalten keine Build-Logik,
sondern rufen Dagger oder lokale Werkzeuge auf.

**Dagger** enthält die eigentliche Pipeline-Logik in Go. Sie läuft lokal und in
der CI identisch — dieselbe Funktion, derselbe Container, dasselbe Ergebnis. Das
ist der Grund für Dagger: kein separates CI-YAML, das nur auf dem Server läuft
und nur dort kaputtgeht.

### Tasks

| Task | Zweck |
|---|---|
| `task up` / `task down` | Compose-Umgebung starten und stoppen |
| `task generate` | templ-Templates und Query-Code generieren |
| `task migrate` | Migrations gegen die lokale DB anwenden |
| `task test` | Unit- und Repository-Tests, einzeln aufrufbar |
| `task lint` | Dagger-Lint |
| `task build` | Dagger-Build, erzeugt Binary und Image |
| `task verify` | Dagger-Verify: End-to-End gegen das gebaute Image |
| `task ci` | lint + test + build + verify, wie in der Pipeline |

### Dagger-Funktionen

**`lint`** — `golangci-lint`, `templ fmt --check`, `go vet`, Prüfung auf
uncommittete Änderungen nach `go generate` (fängt vergessene Regenerierung von
templ-Dateien).

**`build`** — Cross-Build des statischen Binaries, Bau des Container-Images,
Ausgabe als OCI-Artefakt. Version aus Git-Tag bzw. Commit-SHA.

**`verify`** — Startet Postgres als Dagger-Service, wendet Migrations an, fährt
das gebaute Image hoch, prüft `/healthz` und fährt einen End-to-End-Pfad durch:
zwei Spieler anlegen, Match eintragen, bestätigen, Rangliste prüfen.

Der Verify-Schritt ist der eigentliche Wert der Dagger-Entscheidung: Er testet
**das gebaute Image**, nicht den Quellcode. Damit fallen Fehler auf, die ein
`go test` nie sieht — fehlende Migrations im Image, kaputte Env-Defaults,
Templates, die im Container nicht eingebettet sind.

### Reihenfolge in `task ci`

```
lint  →  test  →  build  →  verify
```

Die beiden quellcodenahen Schritte laufen zuerst, weil sie die billigen sind:
ein fehlgeschlagener Unit-Test soll nicht erst auf einen Image-Bau warten.
Verify hängt vom Build-Artefakt ab, nicht vom Quellcode, und steht deshalb am
Ende. Wenn ein Schritt scheitert, brechen die folgenden ab.

**`test` und `verify` messen nicht dasselbe.** Verify fährt genau einen Pfad
durch das gebaute Image und sieht nur, was dieser Pfad berührt. Die Tests
erreichen die Fälle, durch die sich ein Browser nicht sinnvoll führen lässt —
jede einzelne Ablehnungsart bei der Ergebniseingabe, ein Rollback, eine
Wertung, die sich um null bewegt.

## Offene Punkte

Bewusst noch nicht entschieden — gehören in Issues, nicht in ADRs:

- Startwert für neue Spieler. Aktuell 1000 angesetzt. Ob das für eine Bürogruppe
  gut streut, zeigt sich erst mit echten Daten.

Entschieden, seit dieser Abschnitt geschrieben wurde: `points_to_win` ist
konfigurierbar (11 als Vorgabe, 21 als Option), die Images liegen auf ttl.sh
zum Herumzeigen und auf ghcr.io als Artefakt, und `disputed` löst sich über
die Korrektur in AP5 auf.
