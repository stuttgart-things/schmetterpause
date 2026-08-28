# ADR-0009: Container-Image mit ko, Promotion statt zweitem Bau

- **Status:** accepted
- **Datum:** 2026-08-28
- **Betrifft:** Build, Deployment, CI

## Kontext

Für dasselbe Image gab es zwei Rezepte: das `Dockerfile`, das Compose baute,
und `Image()` in `dagger/main.go`, das die Pipeline baute. Ein Kommentar im
Code hielt fest, dass beide übereinstimmen müssen — eine Regel, die niemand
prüft. Go-Version, Base-Image, `ldflags`, Nutzer und Entrypoint standen doppelt
da und mussten von Hand synchron gehalten werden. Invariante 1 ("ein Binary,
ein Image") war damit eine Absicht, keine Eigenschaft des Repos.

Dazu kam die Reihenfolge im Release: Der `release`-Job baute das Image ein
zweites Mal, nachdem der `ci`-Job es gebaut, verifiziert und wieder verworfen
hatte. Anderer Job, anderer Cache, andere Digest — was in GHCR landete, hatte
kein Schritt der Pipeline je angesehen.

Ein Schwachstellen-Scan fehlte vollständig.

## Entscheidung

1. **ko ist der einzige Bau-Pfad.** `.ko.yaml` hält Base-Image, Plattformen und
   `ldflags`. `Dockerfile` und `.dockerignore` entfallen.
2. **Compose baut nicht mehr selbst**, sondern zieht ein Image. `task up` baut
   es vorher lokal mit ko in den Docker-Daemon (`task image`); `SP_IMAGE=...`
   startet stattdessen ein veröffentlichtes Image, ohne irgendetwas zu bauen.
3. **Die Kette ist ttl.sh → verify → trivy → GHCR.** Das Image wird einmal
   gebaut und in die Wegwerf-Registry geschoben; verify und der Scan arbeiten
   auf dieser Referenz. Nach GHCR kommt es per `crane copy` — eine Kopie, kein
   zweiter Bau. Ausgeliefert wird genau die Digest, die geprüft wurde.
4. **Die Pipeline importiert die Blueprint-Module** aus
   `stuttgart-things/dagger` (`go`, `trivy`, `crane`), statt ko, Trivy und
   crane selbst zu verdrahten. Verdrahtet bleiben nur die Flags, die die
   Blueprint-Funktionen nicht durchreichen.

Invariante 1 bleibt unverändert und wird durch diese Entscheidung erstmals
tatsächlich eingehalten. Es wird kein ADR abgelöst.

## Konsequenzen

- **Positiv:** Ein Rezept statt zwei. Base-Image, Go-Version und `ldflags`
  stehen an genau einer Stelle.
- **Positiv:** Compose startet dasselbe Image, das nach Kubernetes und ACA
  geht — vorher war es nur dasselbe Rezept, und auch das nur bei Disziplin.
- **Positiv:** Was ausgeliefert wird, ist die Digest, die verify und der Scan
  gesehen haben.
- **Positiv:** Trivy blockt HIGH und CRITICAL, bevor ein Image nach GHCR
  kommt. `--accept` ist die Notausfahrt für eine Schwachstelle ohne Fix, und
  die Funde stehen dann trotzdem im Log.
- **Negativ:** `task ci` braucht jetzt Netz und schiebt bei jedem Lauf ein
  Image nach ttl.sh. Ein reiner Offline-Lauf ist nicht mehr möglich. Der Preis
  dafür ist, dass verify und Scan dasselbe Artefakt sehen.
- **Negativ:** ttl.sh ist öffentlich und anonym. Für dieses Image ist das
  vertretbar — statisches Binary, eingebettete Assets, keine Credentials, keine
  Daten (Issue #20). Sobald das Image etwas enthält, das nicht jeder lesen
  darf, trägt diese Kette nicht mehr und die Zwischenstufe muss in eine
  PR-Registry in GHCR wandern.
- **Negativ:** ko setzt kein `CMD`. Der Binary-Pfad wandert von
  `/usr/local/bin/schmetterpause` nach `/ko-app/schmetterpause`. Der
  Compose-Healthcheck ist angepasst; wer den alten Pfad anderswo stehen hat
  (K8s-Probe, ACA-Command), muss ihn nachziehen. Dass `serve` der
  Default-Unterbefehl ist, macht den fehlenden `CMD` folgenlos.
- **Negativ:** `EXPOSE 8080` entfällt. Das war reine Metadatenangabe; Compose
  und K8s mappen den Port ohnehin selbst.
- **Negativ:** ko ist an zwei Stellen gepinnt — `KO_VERSION` im Taskfile für
  den lokalen Bau, `koVersion` in `dagger/main.go` für die Pipeline. Renovate
  hält beide auf demselben Stand; auseinanderlaufen dürfen sie nicht.

## Verworfene Alternativen

- **Dockerfile behalten, nur die Promotion einführen.** Löst das Doppelrezept
  nicht, und genau daraus entstand die Drift.
- **ko nur für Kubernetes, Dockerfile für Compose.** Hätte Invariante 1
  ausdrücklich aufgeweicht: Compose würde dann ein Image testen, das nirgends
  läuft.
