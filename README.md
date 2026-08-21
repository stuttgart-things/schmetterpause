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
task ci        # lint, build, verify — identisch zur Pipeline
```

Alle Befehle laufen über [Task](https://taskfile.dev/). Die eigentliche
Pipeline-Logik steckt in [Dagger](https://dagger.io/) und läuft lokal wie in der
CI gleich — `task ci` ist kein Näherungswert, sondern derselbe Code.

`task --list` zeigt alle verfügbaren Tasks.

## Dokumentation

- `CLAUDE.md` — Invarianten, fachliche Begriffe, Konventionen. Vor Änderungen lesen.
- `docs/mvp-plan.md` — Scope, Datenmodell, Arbeitspakete, Definition of Done.
- `docs/adr/` — Architekturentscheidungen samt verworfener Alternativen.

Eine Änderung, die einem ADR widerspricht, braucht ein neues ADR mit
`supersedes`-Verweis.

## Zum Namen

Schmetterball plus Pause. Der Name entstand aus einer Abstimmung im Büro und
gilt als gesetzt — er steckt im Modulpfad, im Image-Namen und im Browser-Titel.
