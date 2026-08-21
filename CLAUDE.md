# Schmetterpause

Matchmaking-, Liga- und Turnier-App für Büro-Tischtennis. Go + templ + HTMX,
Postgres, ein Container.

## Invarianten

Diese Regeln gelten für jede Änderung. Wenn eine Aufgabe sie verletzt, erst
nachfragen — nicht stillschweigend abweichen.

1. **Ein Binary, ein Image.** Dasselbe Image läuft in Docker Compose, Kubernetes
   und Azure Container Apps. Keine umgebungsspezifischen Builds, keine
   Build-Tags für Deployment-Ziele.
2. **Konfiguration ausschließlich über Environment-Variablen.** Keine
   Config-Dateien im Image, keine hartkodierten Hosts, Ports oder URLs. Defaults
   gehören in den Code, nicht in eine mitgelieferte Datei.
3. **Kein Redis**, solange keiner der in `docs/adr/0002` genannten Auslöser
   eingetreten ist.
4. **Auth-Provider liegen hinter einer Schnittstelle.** Kein Handler kennt
   GitLab, GitHub oder WebAuthn direkt. Die Anwendung arbeitet mit `player_id`.
5. **Datenzugriff über Repository-Interfaces, kein SQL in Handlern.** Hält den
   in `docs/adr/0001` erwähnten Wechselpfad offen und macht Handler testbar.
6. **Keine biometrischen Daten verlassen jemals das Gerät des Nutzers.** Siehe
   `docs/adr/0004`.
7. **Kein JavaScript-Framework.** Interaktivität über HTMX. Handgeschriebenes JS
   nur, wo HTMX nicht ausreicht, und dann minimal.
8. **Migrations sind vorwärtsgerichtet und additiv.** Keine destruktiven
   Änderungen ohne separaten Schritt.

## Architekturentscheidungen

Vor Änderungen an Datenmodell, Auth oder Deployment die ADRs in `docs/adr/`
lesen. Eine Entscheidung, die einem ADR widerspricht, braucht ein neues ADR mit
`supersedes`-Verweis — kein stiller Umbau.

## Fachliche Begriffe

- **TTR** — Tischtennis-Rating nach dem deutschen Verbandssystem.
  Siegwahrscheinlichkeit `P(A) = 1 / (1 + 10^((TTR_B - TTR_A) / 150))`,
  Divisor 150, nicht 400 wie bei Schach-Elo. Änderungskonstante 16 als Basis.
- **Veranstaltungsweise Wertung** — bei Turnieren und Ligarunden werden die
  Erwartungswerte über alle Einzel eines Spielers summiert und einmal
  verrechnet, nicht nach jedem Match. Das macht das Ergebnis unabhängig von der
  Reihenfolge der Ergebniseingabe.
- **Nur Einzel zählen für TTR.** Doppel brauchen, falls sie kommen, eine eigene
  Wertung.
- **Slot** — buchbares Zeitfenster an einer Platte. Existiert unabhängig vom
  Turniermodell; Turnierspiele belegen Slots über denselben Mechanismus wie
  Casual-Buchungen.

## Konventionen

- Fachlogik (TTR-Berechnung, Tabellenberechnung, Bracket-Fortschreibung) liegt
  in eigenen Packages ohne Datenbank- oder HTTP-Abhängigkeit und ist per
  Unit-Test abgedeckt.
- Handler geben HTML-Fragmente zurück, keine JSON-APIs, solange kein externer
  Konsument existiert.
- Fehler werden gewrappt (`fmt.Errorf("...: %w", err)`), nicht verschluckt.
- **Commits folgen Conventional Commits.** Der Ablauf steht in
  `.claude/skills/conventional-commits`: kleine Commits, jeweils eine logische
  Änderung, Nachricht als `typ(scope): beschreibung` im Imperativ.
- **Branches tragen denselben Typ als Präfix** wie der Commit, der sie prägt —
  `feat/`, `fix/`, `refactor/`, `ci/`, `docs/`, `chore/`, gefolgt von einem
  kurzen Bezeichner. `feat/ttr-package`, nicht `wip` und nicht der Name dessen,
  der ihn angelegt hat.
- **Code ist englisch, Dokumentation und Oberfläche sind deutsch.** Kommentare,
  Bezeichner, Log- und Fehlermeldungen im Code, Commit-Nachrichten, Issues und
  Pull Requests auf Englisch. `CLAUDE.md`, `docs/` und alle Texte, die ein
  Spieler im Browser sieht, bleiben deutsch.
