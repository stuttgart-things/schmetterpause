# Getting a player back in

A player cannot sign in. Before anything else, one rule: **nobody sets a PIN for
somebody else, and admins are no exception**
([ADR-0007](adr/0007-pin-als-anmeldung.md),
[ADR-0008](adr/0008-wer-fuer-andere-handeln-darf.md)). A PIN that somebody else
knows is not a PIN. `/admin` has no button for it, and SQL is not the way round
it. The player always sets their PIN themselves. What we can do is help them
get back to the point where they can set it.

This page is the how. It was written on 2026-09-21, when the office needed it
for the first time and had to switch the kiosk on first. The record of that is
at the end.

## Which case is it?

| The player still has | The way back |
| --- | --- |
| a device that is still signed in | profile page → **PIN** → set a new one. Done. |
| their recovery code | sign in with the code in the *PIN oder Wiederherstellungscode* field, then set a new PIN on the profile page |
| neither | the kiosk issues a new recovery code, [below](#at-the-table) |

The third case is the only one that needs an operator. The proof of identity
is the room: the player stands next to the kiosk, and the people there know
them ([ADR-0006](adr/0006-wiederherstellungscode.md)). That is why this does
not work remotely, and why nothing in this page tries to make it work
remotely.

## Is the kiosk on?

Look at **`/admin` → *Diese Instanz* → *Kiosk am Tisch***. It says **an** or
**aus**. The page reads the value when the application starts, and it shows
the start time underneath. A kiosk that was just switched on in a manifest
stays *aus* until the pod restarts. Versions up to v0.12.0 do not have this
section.

If you have no admin session, or the version is older, run `curl -s -o /dev/null -w '%{http_code}'
https://<host>/kiosk`: **200** means on, **404** means off.

## Switching the kiosk on

The application needs `SP_KIOSK_TOKEN`. If it is unset, `/kiosk` is not even
registered. How the variable gets there depends on the environment:

| Environment | Setting |
| --- | --- |
| Docker Compose | `SP_KIOSK_TOKEN` in `.env` (`task office:setup` asks for it), then `task office:up` |
| kcl | `-D config.kioskEnabled=true` + property `kiosk-token` in the Vault entry |
| Terraform (Azure) | `kiosk_token` in `terraform.tfvars` |
| argocd catalog | `kiosk.enabled: true` in the schmetterpause values (argocd#514) |

### On homerun2-test1

On homerun2-test1 there are four steps, and **every one of them has to be
in place**:

1. **The Vault entry** `schmetterpause/schmetterpause-kiosk`, property `token`.
   It has existed since 2026-09-21, so this step is only needed if the entry
   was deleted. Use a token of its own, never one of the other entries:
   ```sh
   curl -sS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
     https://vault.infra.sthings-vsphere.labul.sva.de/v1/schmetterpause/data/schmetterpause-kiosk \
     -d "$(jq -n --arg t "$(openssl rand -hex 16)" '{data:{token:$t}}')"
   ```
2. **Read access for the office.** The policy `read-schmetterpause` names
   every entry it grants, one path per entry. An entry the policy does not
   name is refused with **403**. The policy lives in `stuttgart-things`
   `clusters/labul/vsphere/infra-sthings/vault-schmetterpause-secrets/terraform.tfvars.sops.json`
   (`kv_policies`). It has included the kiosk path since stuttgart-things#3104.
3. **The switch** `kiosk.enabled: true` in `stuttgart-things`
   `clusters/labul/vsphere/platform-sthings/argocd/homerun2-test1/tabletennis.yaml`
   (stuttgart-things#3101). Argo appends `SP_KIOSK_TOKEN` to the ExternalSecret
   `schmetterpause-app`.
4. **A restart.** The Deployment does not restart by itself when the Secret
   changes:
   ```sh
   kubectl -n schmetterpause rollout restart deploy/schmetterpause
   kubectl -n schmetterpause rollout status deploy/schmetterpause
   ```
   The office has one replica with the `Recreate` strategy, so it is
   unreachable for a few seconds.

Check each step before you move on:

```sh
# 2 and 3: the ExternalSecret synced, and the Secret holds the key (names only)
kubectl -n schmetterpause get externalsecret schmetterpause-app
kubectl -n schmetterpause get secret schmetterpause-app -o json | jq -r '.data | keys[]'
# SP_KIOSK_TOKEN
# SP_SCOREBOARD_TOKEN
# SP_SESSION_KEY

# 4: the kiosk exists
curl -s -o /dev/null -w '%{http_code}\n' https://<host>/kiosk   # 200
```

**The trap in step 2.** A kiosk key that cannot be read stops the **whole**
ExternalSecret, not just the kiosk: `SecretSyncedError`, `403 permission
denied`. The running application keeps working, because ESO leaves the last
Secret in place. But while the sync fails, nothing else in that Secret can
change either, the session key included. To force a new sync after fixing
the policy:

```sh
kubectl -n schmetterpause annotate externalsecret schmetterpause-app \
  force-sync=$(date +%s) --overwrite
```

## At the table

You need the kiosk token once. Put it straight into the clipboard and do not
print it into the terminal history:

```sh
kubectl -n schmetterpause get secret schmetterpause-app \
  -o jsonpath='{.data.SP_KIOSK_TOKEN}' | base64 -d | xclip -sel clip
```

Without cluster access you can read it from Vault instead:
`curl -sS -H "X-Vault-Token: $VAULT_TOKEN" https://vault.infra.sthings-vsphere.labul.sva.de/v1/schmetterpause/data/schmetterpause-kiosk | jq -r .data.data.token | xclip -sel clip`.

1. **Unlock the machine.** On the device at the table, open
   `https://<host>/kiosk?token=<token>`. The token becomes a cookie and
   disappears from the address bar. The device stays a kiosk for twelve hours.
2. **Name who is typing.** The kiosk asks this before it offers anything else
   ([ADR-0014](adr/0014-kiosk-benennt-wer-eintraegt.md)). An observer such as
   `timoboll` may be the operator.
3. **Issue the code.** Go to *Zugang wiederherstellen*, choose the player, and
   press **Neuen Code ausgeben**. The code is shown **once, on this screen
   only**. From this moment the player's old code no longer works.
4. **The player signs in** on their own phone: *Anmelden*, their name, and the
   code in the *PIN oder Wiederherstellungscode* field.
5. **The player sets their PIN** on their profile page: at least six digits,
   chosen by them, and not seen by anybody else.

The log records step 3 as `recovery code issued at the kiosk`, with the
player's id and the operator's id.

## Afterwards

- **Take the device back**: go to `/admin` → *Freigeschaltete Kiosk-Geräte* →
  **Zurücknehmen**. Otherwise it stays a kiosk until the twelve hours run out,
  and anybody who walks past it can enter results for everybody.
- **Leave the kiosk on or switch it off.** Leaving it on is fine. The token is
  the lock, and no device is a kiosk until somebody types the token. To switch
  it off, set `kiosk.enabled: false`, restart the pod, and check that
  *Diese Instanz* says **aus**. The Vault entry and the policy line can stay:
  they cost nothing and save steps 1 and 2 next time.

## Record: homerun2-test1, 2026-09-21

| Step | What happened |
| --- | --- |
| Case | A player had neither a PIN nor a recovery code, and no device signed in. The kiosk was off. |
| Vault entry | `schmetterpause-kiosk` created at 13:24Z |
| Switch | argocd#514 added `kiosk.*` to the catalog chart; stuttgart-things#3101 set `enabled: true` |
| 403 | The ExternalSecret failed with `permission denied` on `schmetterpause/data/schmetterpause-kiosk`. The app kept running on the old Secret. |
| Policy | The kiosk path was added to `read-schmetterpause` live through the API, then in the tfvars (stuttgart-things#3104). `terraform plan` showed no changes. |
| Restart | `rollout restart`; `/kiosk` answered 200 |
| At the table | Still to be done at the time of writing |
