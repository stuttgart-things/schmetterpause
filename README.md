# Schmetterpause

Matchmaking-, Liga- und Turnier-App für Büro-Tischtennis.

Go + templ + HTMX, Postgres, ein Container. Läuft in Docker Compose, Kubernetes
und Azure Container Apps aus demselben Image.

## Status

Im Aufbau. Aktueller Meilenstein ist der MVP nach `docs/mvp-plan.md`: Ergebnisse
erfassen, vom Gegner bestätigen lassen, TTR berechnen, Rangliste anzeigen.
Liga-, Turnier- und Buchungsmodus kommen später.

## Loslegen

Voraussetzungen: [Task](https://taskfile.dev/), Docker und die
[Dagger-CLI](https://docs.dagger.io/install). Beim ersten Aufruf einer
Dagger-Task einmalig `task dagger:develop` ausführen — das erzeugt die
generierten Teile des Pipeline-Moduls.

```sh
task up        # Compose-Umgebung starten (App + Postgres)
task run       # App lokal gegen die Compose-Datenbank, ohne Container
task ci        # lint, test, build, verify — identisch zur Pipeline
task office:setup # .env für einen Abend an der Platte
task office:up    # starten, im Netz erreichbar
```

`task up` startet das **veröffentlichte** Image aus GHCR, nicht eines aus dem
Arbeitsverzeichnis — dasselbe Artefakt, das im Cluster läuft. Für den eigenen
Stand gibt es `task run` (schnell, ohne Container) und `task up:local` (baut das
Image mit Dagger und startet Compose damit).

Alle Befehle laufen über [Task](https://taskfile.dev/). Die eigentliche
Pipeline-Logik steckt in [Dagger](https://dagger.io/) und läuft lokal wie in der
CI gleich — `task ci` ist kein Näherungswert, sondern derselbe Code.

`task --list` zeigt alle verfügbaren Tasks.

## Dokumentation

Alles aus `docs/` steht als Website unter
**<https://stuttgart-things.github.io/schmetterpause/>** — mit Navigation und
Suche, und damit die bessere Adresse für alles, was jemand im Stehen oder auf
dem Handy liest. Diese Datei bleibt der Einstieg für alle, die das Repository
ohnehin ausgecheckt haben.

- `CLAUDE.md` — Invarianten, fachliche Begriffe, Konventionen. Vor Änderungen lesen.
- `docs/mvp-plan.md` — Scope, Datenmodell, Arbeitspakete, Definition of Done.
- `docs/adr/` — Architekturentscheidungen samt verworfener Alternativen. Der
  Index unter `docs/adr/index.md` wird generiert: `task docs:index`.
- `docs/turnier-vor-ort.md` — ein Abend an der Platte mit einem Laptop für alle.

Eine Änderung, die einem ADR widerspricht, braucht ein neues ADR mit
`supersedes`-Verweis.

## Zum Namen

Schmetterball plus Pause. Der Name entstand aus einer Abstimmung im Büro und
gilt als gesetzt — er steckt im Modulpfad, im Image-Namen und im Browser-Titel.
