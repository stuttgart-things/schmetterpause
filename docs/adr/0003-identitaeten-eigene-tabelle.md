# ADR-0003: Identitäten in eigener Tabelle, getrennt vom Spieler

- **Status:** accepted
- **Datum:** 2026-08-21
- **Betrifft:** Datenmodell, Authentifizierung

## Kontext

Die App startet ohne echte Authentifizierung: Ein Spieler legt sich mit einem
Anzeigenamen an und wird über ein signiertes Cookie wiedererkannt. Geplant sind
später OIDC gegen das Firmen-GitLab (Codehub), ggf. GitHub, sowie Passkeys (siehe
ADR-0004). Diese Verfahren sollen nebeneinander existieren, und ein Spieler soll
mehrere davon nutzen können, ohne seine Historie zu verlieren.

## Betrachtete Optionen

1. Provider-Spalten direkt auf `players` (`github_id`, `gitlab_id`, ...)
2. Eigene Tabelle `identities` mit `(provider, subject)` als Schlüssel
3. Anzeigename als Identität

## Entscheidung

Wir führen eine eigene Tabelle:

```sql
identities (
  provider   text,    -- 'gitlab' | 'github' | 'passkey' | 'local'
  subject    text,    -- sub aus dem ID-Token bzw. Credential-ID
  player_id  uuid references players(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (provider, subject)
)
```

`players` enthält ausschließlich fachliche Daten (Anzeigename, TTR,
Erstellungszeitpunkt) und keine Auth-Attribute. Die Anwendung arbeitet
durchgängig mit `player_id`; wie diese ermittelt wurde, ist außerhalb des
Auth-Layers unsichtbar.

## Begründung

- Jeder neue Provider ist ein Datensatz, kein Schema-Change. Provider-Spalten auf
  `players` würden bei jedem weiteren Verfahren eine Migration erzwingen und die
  Tabelle mit überwiegend `NULL`-Werten füllen.
- Mehrere Identitäten pro Spieler sind ohne Sonderfall abbildbar — notwendig,
  weil ein Passkey ein zusätzlicher Faktor neben dem GitLab-Login ist und nicht
  dessen Ersatz.
- Die Umstellung vom anonymen Namensmodus auf echtes Login wird zu einem
  `INSERT` in `identities`. TTR, Match-Historie und Statistiken bleiben
  unverändert am bestehenden `player_id` hängen.

## Konsequenzen

- **Positiv:** Anonyme Anlage, späteres Login und Account-Verknüpfung sind
  derselbe Mechanismus.
- **Negativ:** Ein zusätzlicher Join bei jedem Login. Bei dieser Datenmenge
  irrelevant.
- **Offen:** Das Zusammenführen zweier versehentlich getrennt angelegter Spieler
  (jemand legt sich anonym an und loggt sich später neu über GitLab ein, ohne die
  bestehende Identität zu verknüpfen) ist damit nicht gelöst. Dafür braucht es
  eine bewusste Merge-Funktion. Wird bei Bedarf als eigenes ADR behandelt.

## Verworfene Alternativen

Provider-Spalten auf `players` wären für genau einen Provider einfacher und
wurden in einer frühen Skizze auch so vorgeschlagen. Verworfen, sobald klar war,
dass mindestens drei Verfahren koexistieren sollen.

Anzeigename als Identität ist ausgeschlossen. Namen sind nicht eindeutig,
änderbar und würden bei jeder Umbenennung die Historie brechen.
