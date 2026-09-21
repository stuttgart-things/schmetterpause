# Admins and observers

The rules are in two ADRs. [ADR-0008](adr/0008-wer-fuer-andere-handeln-darf.md)
gives some people the right to act for others, as a flag on their own account.
[ADR-0022](adr/0022-beobachter-spielt-nicht.md) adds accounts that never play.
This page is the how. Everything on it was run on homerun2-test1 on 2026-09-16,
when the office got its first admin; the record is at the end.

## Two flags, independent of each other

| Flag | What it means | Who sets it |
| --- | --- | --- |
| `is_admin` | May act for other people under `/admin` | `SP_BOOTSTRAP_ADMIN` at start |
| `is_observer` | Does not play | An admin under `/admin` |

An account may hold either, both or neither. The office's own admin,
`timoboll`, holds both: it corrects results and never shows up in the ranking.

## What an admin can do

Everything is under `/admin`, and only in the admin's own signed-in session.
Every action writes a log line with the admin's `player_id`.

- **Take back a counted result** (*Gewertete Ergebnisse*). Both ratings return
  to their values before the match. Only the **newest** result of both players
  can go. If either has played since, the page refuses and says to remove the
  newer one first, because writing the old ratings back would silently undo
  the later match too.
- **Remove a player** (*Spieler ohne Ergebnis*): a joke entry or a duplicate.
  Anybody who played, reported a result or started a tournament cannot be
  removed. The database refuses, and the page says so. Their sign-in, PIN and
  recovery code go with them. You cannot remove yourself.
- **Take kiosk machines back** (*Freigeschaltete Kiosk-Geräte*), one or all.
- **Mark somebody as an observer**, or let them play again (below).

An admin **cannot**:

- set or read anybody's PIN (ADR-0007, ADR-0008). Somebody who is locked
  out gets a new recovery code at the kiosk instead; see
  [Getting a player back in](access-recovery.md);
- grant or withdraw the admin flag through the page. That only happens through
  `SP_BOOTSTRAP_ADMIN`; see [Open ends](#open-ends).

## What an observer is

An observer signs in like anybody else, but does not play:

- **not in the ranking** or the statistics;
- **not offered** as opponent, kiosk player, tournament player or in
  `GET /api/players`, so the Zählwerk does not offer them either;
- **refused as a player** on every write path. The form, the kiosk and the
  tournament page answer *„Ein Beobachter spielt nicht mit."*;
  `POST /api/results` answers 422;
- **allowed as the operator** at the kiosk and in `POST /api/results`: somebody
  who watches and counts is exactly what an operator is (ADR-0014).

Their own profile page keeps the name and the access section (PIN, recovery
code), and drops rating, rank and matches. Their start page has no result entry.

**Only somebody who never took part can become an observer**: no match of any
status on either side, and no tournament field. Otherwise their matches would
vanish from other people's rankings. Having reported somebody else's result
does not count as taking part. Letting an observer play again always works;
they come back at their unchanged rating.

## Setting up the first admin

The first admin comes from the environment: `SP_BOOTSTRAP_ADMIN` names a
player by display name, and at every start that player gets the flag. The
match ignores case and surrounding spaces, but not spaces inside the name.
The flag stays in the database; the variable only grants it again.

**1. The person joins first**, through the normal page, and chooses their PIN
themselves. Nobody sets a PIN for somebody else, an operator included. A
variable naming somebody who has not joined yet is a warning in the log, not
a failed start.

**2. Set the variable** where the environment is configured:

| Environment | Setting |
| --- | --- |
| Docker Compose | `SP_BOOTSTRAP_ADMIN` in `.env`, then `task up` / `task office:up` |
| kcl | `-D config.bootstrapAdmin=<name>` |
| Terraform (Azure) | `bootstrap_admin`, as a second apply after the person has joined |
| argocd catalog | `bootstrapAdmin` in the schmetterpause `values` (argocd#470) |

On homerun2-test1 it is the schmetterpause `values` block in
`stuttgart-things/stuttgart-things`
`clusters/labul/vsphere/platform-sthings/argocd/homerun2-test1/tabletennis.yaml`.

**3. Restart the application.** The flag is granted at start, and the
catalog's ConfigMap has a fixed name, so a changed value does **not** roll the
Deployment by itself:

```sh
kubectl -n schmetterpause rollout restart deploy/schmetterpause
```

Argo may undo the restart annotation and roll the pod once more. That is
harmless: the flag is already in the database.

**4. Check the log** for the grant:

```json
{"level":"INFO","msg":"admin flag granted from SP_BOOTSTRAP_ADMIN","player_id":"…","display_name":"…"}
```

The header then shows the admin a **Rechte** link, which leads to `/admin`.

## Setting up an admin who never plays

The office's setup: an account that administers and is an observer. **The
order matters.**

1. **Deploy a version with observers first** (v0.11.0 or later). Until then the
   account would be an ordinary player.
2. **Join** as the account, with its own PIN.
3. **Nobody plays it.** Between joining and step 5 the account sits at the
   bottom of the ranking with TTR 1000 and can be picked as an opponent. One
   match — even a pending one — and it can never become an observer.
4. **Make it admin**, as in [Setting up the first admin](#setting-up-the-first-admin).
5. **Signed in as that account**, open `/admin` and press **Beobachter** on
   your own row under *Spieler ohne Ergebnis*, the one marked *(du)*. The page
   answers *„… ist jetzt Beobachter: nicht in der Rangliste, nicht wählbar,
   meldet sich aber weiter an."* and the account moves to the *Beobachter*
   section.

To check:

```sh
# the account is gone from the Zählwerk's list
curl -s -H "Authorization: Bearer $SP_SCOREBOARD_TOKEN" \
  https://<host>/api/players | jq 'map(.display_name) | index("<name>")'
# null
```

and the log shows `"msg":"observer flag set","observer":true` with the
admin's own id as `by`.

## Other observers

An admin can make any player without a history an observer, for example a
colleague who only watches. Press **Beobachter** on their row. **Spielt wieder
mit** in the *Beobachter* section undoes it. If the player has a history, the
page refuses and says why. There is no way around it, and there should not be:
a player with matches who stops playing simply stops playing.

## Why the Zählwerk has no account

It would be easy to give the Zählwerk an observer account and put that in
`reported_by`. [ADR-0022](adr/0022-beobachter-spielt-nicht.md) decided
against it. The machine is already identified by its token and by
`entered_via = 'scoreboard'`, and `reported_by` names the person who watched.
An account would erase that person and put a PIN into a device.

## Open ends

- **No page grants or withdraws the admin flag.** A second admin means naming
  them in `SP_BOOTSTRAP_ADMIN` for one restart; the first keeps the flag.
  Withdrawing the flag today means SQL. ADR-0008 lists a page for this as open
  point 1, to be built once the variable stops being enough.
- **The PIN is the admin's only secret.** At least six digits, rate-limited (ADR-0007).
  ADR-0008 open point 2 says an admin probably needs more once passkeys exist.

## Record: homerun2-test1, 2026-09-16

| Step | What happened |
| --- | --- |
| Deploy | v0.11.0 (stuttgart-things#3009). Migration `20260916170000` ran in 17 ms, after backup `20260916T155148` at 16 players / 69 matches |
| Join | `timoboll` joined with its own PIN, 0 matches |
| Admin | `bootstrapAdmin: timoboll` (argocd#470, stuttgart-things#3010), manual restart, `admin flag granted from SP_BOOTSTRAP_ADMIN` in the log |
| Observer | `timoboll` pressed **Beobachter** on its own row, `observer flag set` with its own id as `by` |
| Check | Ranking 16 rows without `timoboll`, `/api/players` 16, Zählwerk 16, profile 200 with *„Beobachter — spielt nicht mit"*, 17 players / 69 matches in total |
