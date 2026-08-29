# Kubernetes-Manifeste

KCL-Modul für den Betrieb von Schmetterpause auf Kubernetes. Rendert
ServiceAccount, ConfigMap, zwei ExternalSecrets, Deployment, Service und zwei
HTTPRoutes — je nachdem, was das Profil einschaltet.

Alles ist eine Variable. Kein Hostname, kein Namespace, kein Gateway und kein
Image-Verweis steht fest im Modul, damit dieselben Manifeste für einen
Laptop-Cluster, den Büro-Cluster und einen Preview-Namespace passen.

## Rendern

```sh
task kcl:render                      # Profil cicd-test2
task kcl:render PROFILE=existing-secrets
task kcl:check                       # rendern alle Profile noch?
task kcl:kustomize                   # kustomize-Base nach build/kustomize
task kcl:publish TAG=v1.2.3          # Base als OCI-Artefakt nach GHCR
```

Direkt mit der CLI, wenn es genauer sein soll:

```sh
kcl run kcl/main.k -Y kcl/profiles/cicd-test2.yaml \
  -D 'config.image=ghcr.io/stuttgart-things/schmetterpause:v1.2.3' \
  -D 'config.kioskEnabled=true'
```

### Warum die Ausgabe so aussieht, wie sie aussieht

`kcl run` liefert eine Liste unter dem Schlüssel `manifests:`. Das ist kein
Schönheitsfehler, sondern ein **Vertrag**: `stuttgart-things/dagger/kcl` — das
Modul, das daraus eine kustomize-Base baut und sie als OCI-Artefakt schiebt —
normalisiert die Ausgabe mit einer festen `sed`-Kette, die genau diese Form
erwartet: erste Zeile weg, zwei Stellen ausrücken, `- ` am Zeilenanfang wird
zum Dokumententrenner.

Ein YAML-Strom (`manifests.yaml_stream`) sieht schöner aus und ist direkt
`kubectl apply -f`-tauglich — aber durch dieselbe Kette geschickt verliert die
erste Ressource ihr `apiVersion`, und jedes `metadata`-Feld rutscht auf die
oberste Ebene. Das Ergebnis parst weiterhin als YAML, es deployt nur etwas
anderes. Nachgemessen, nicht vermutet.

**Deshalb `task kcl:render` statt `kcl run`.** Der Task wendet dieselbe
Normalisierung an, damit lokal genau das entsteht, was später im OCI-Artefakt
liegt. Die Ausgabe ist dann `---`-getrennt und für `kubectl apply -f` brauchbar.

**Ein Render ohne Profil scheitert**, und zwar mit Absicht: `secretsMode` steht
auf `external`, und dafür braucht es einen Secret-Store und einen Pfad. Die
Fehlermeldung nennt beide. Ein Modul, das ohne Angaben irgendetwas rendert,
rendert etwas, das niemand deployen will.

## Was nicht gerendert wird

**Kein Namespace.** Mehrere Applications teilen sich einen Workload-Namespace.
Schickt mehr als eine davon eine Namespace-Ressource mit, markiert ArgoCD sie
als SharedResource — und der Prune-Lauf der einen löscht den Namespace unter den
anderen weg. Stattdessen setzt die konsumierende Application
`CreateNamespace=true`.

**Kein CloudNativePG-Cluster.** Aus einem verwandten Grund mit schlimmeren
Folgen: die Datenbank überlebt jede Revision der Anwendung. Eine Base, die den
`Cluster` trägt, ist eine Base, deren Entfernung die Daten mitnehmen kann. Der
`Cluster` ist eine eigene Argo-Application aus `infra/cloudnative-pg/cluster`
im Katalog, in denselben Namespace, mit niedrigerer Sync-Wave — damit er
existiert, bevor der Migrations-initContainer läuft.

## Zwei Dinge, die anders sind, als sie aussehen

### Die zweite HTTPRoute ist keine Zierde

Das geteilte Gateway bedient denselben Wildcard auf zwei Listenern: HTTPS auf
443 und Klartext-HTTP auf 80. Eine Route ohne `sectionName` hängt sich an
**beide**. Über Klartext-HTTP schickt ein Browser das `Secure`-Session-Cookie
nie zurück — der Spieler tritt bei, bekommt seinen Wiederherstellungscode
einmal gezeigt und ist beim nächsten Request wieder fremd. Das ist Issue #70,
nachgebaut von einem Listener, und es sähe aus wie ein Anwendungsfehler.

Also bindet die Anwendung namentlich an den TLS-Listener, und der
Klartext-Listener bekommt eine eigene Route, die nur weiterleitet.

### Kein Feld kann ein Geheimnis enthalten

Im Schema gibt es keinen Platz für `SP_SESSION_KEY`, und keinen Default, der
einen erfinden könnte. Was das Schema annimmt, ist ein **Pfad** in den
Secret-Store.

Der Grund ist nicht Ordnungsliebe: ein geänderter Session-Key scheitert nicht,
er vergisst still jeden Spieler — `TestARestartWithADifferentKeyForgetsEverybody`
hält das fest. Ein Wert in einer Profildatei ist ein Wert in Git, und ein
gerendertes Secret mit erzeugtem Default würde das Büro bei jedem Render
ausloggen. Ein falscher Vault-Pfad scheitert dagegen laut, beim Sync. Laut ist
hier besser als still.

Zwei Secrets statt einem, und die Aufteilung ist Least Privilege: der
Migrations-initContainer bekommt nur das Datenbank-Secret und sieht das
Cookie-Geheimnis nie. Dass `config.Load` und `ValidateForServe` getrennt sind,
macht das kostenlos.

| Secret | Schlüssel | gelesen von |
|---|---|---|
| `<name>-db` | `username`, `password`, `SP_DATABASE_URL` | initContainer **und** App |
| `<name>-app` | `SP_SESSION_KEY`, optional `SP_KIOSK_TOKEN` | nur die App |

`<name>-db` ist vom Typ `kubernetes.io/basic-auth`, damit CloudNativePG es über
`appSecretName` als Zugangsdaten des Owners beim Bootstrap nehmen kann. Die
`SP_DATABASE_URL` baut das ESO-Template daraus zusammen: das Passwort kommt aus
Vault, Host, Port, Datenbank und `sslmode` kommen aus diesem Modul.

> **Passwort in Vault alphanumerisch halten.** Es wird ohne Maskierung in die
> URL eingesetzt. Ein Passwort mit `@`, `/`, `:` oder `?` ergibt eine DSN, die
> als etwas anderes gelesen wird.

## Wichtige Werte

Die vollständige Liste steht mit Begründung in `schema.k`. Was man am ehesten
setzt:

| Wert | Default | Bedeutung |
|---|---|---|
| `config.image` | `…:latest` | Für einen echten Deploy einen Digest pinnen |
| `config.namespace` | `schmetterpause` | |
| `config.clusterDomain` | *(leer)* | Ergibt mit `name` den Hostnamen |
| `config.host` | *(leer)* | Überschreibt die Zusammensetzung |
| `config.httpRouteEnabled` | `false` | |
| `config.gatewayName` / `…Namespace` | *(leer)* | |
| `config.httpRedirectEnabled` | `true` | Route auf Port 80, die auf https leitet |
| `config.kioskEnabled` | `false` | Ohne `SP_KIOSK_TOKEN` gibt es den Kiosk nicht |
| `config.secretsMode` | `external` | `existing`, wenn die Secrets anderswoher kommen |
| `config.secretStoreName` | *(leer)* | z. B. `vault` |
| `config.vaultPath` | *(leer)* | Pfad, nie Wert |
| `config.replicas` | `1` | Ein `check:` hält es dort, siehe unten |
| `config.bootstrapAdmin` | *(leer)* | Anzeigename, wirkt beim Start |

### Warum `replicas` bei 1 festgenagelt ist

`internal/repository/postgres/migrate.go` ruft `goose.UpContext` über die
Package-API, und die nimmt **kein** Session-Lock — das gibt es nur auf goose'
Provider-API. Zwei Pods, die gleichzeitig migrieren, sind real unsicher, nicht
theoretisch. Ein `check:` im Schema lehnt alles andere ab. Höher gehen heißt
vorher ein Advisory-Lock oder einen Migrations-Job bauen.

Aus demselben Grund ist die Strategie `Recreate` und nicht `RollingUpdate`: ein
Rolling Update ließe den neuen Pod `migrate up` laufen, während der alte noch
das alte Schema bedient.

## Das Profilformat

Flach, `key: value` — **nicht** KCLs eigenes `kcl_options`-Format:

```yaml
config.namespace: schmetterpause
config.httpRouteEnabled: true
```

Das ist die Form, die `stuttgart-things/dagger/kcl` liest. Es wandelt die Datei
im Container um mit

```sh
yq eval -o=json params.yaml | jq 'to_entries | map(.key + "=" + (.value|tostring))'
```

und macht daraus ein `-D` pro Eintrag. Eine Datei im `kcl_options`-Format
überlebt diese Umwandlung als **ein einziges** `-D kcl_options=[…]`, das KCL
nicht kennt — jeder Wert fällt still auf seinen Default zurück. Bei uns hat
das der `check:` auf `secretStoreName` abgefangen; ohne den wäre ein Artefakt
mit lauter Defaults entstanden, das aussieht wie ein Deploy.

`task kcl:render` macht dieselbe Umwandlung lokal. Deshalb `task kcl:render`
statt `kcl run -Y` — Letzteres will das `kcl_options`-Format und würde eine
Datei akzeptieren, die im Publish-Weg nicht funktioniert.

## Die Kette dahinter

`kcl:kustomize` und `kcl:publish` rufen ein geteiltes Dagger-Modul auf, statt
etwas Eigenes zu bauen — das ist der Weg, den #80 beschreibt:

```
kcl/  ──render-kustomize-base──▶  kustomize-Base
      ──push-kustomize-base────▶  ghcr.io/…/schmetterpause-kustomize:<tag>
                                          │
                                  Argo CD zeigt darauf (#81)
```

Der Tag ist **derselbe wie beim Image**, mit Absicht: ein Artefaktpaar, das
auseinanderlaufen kann, ist ein Deploy, den hinterher niemand mehr rekonstruiert.

Das Modul ist auf `@v0.82.0` gepinnt. Ohne Pin könnte `task kcl:publish` morgen
etwas anderes rendern als heute, und das ist die eine Eigenschaft, die ein
Deploy-Artefakt nicht haben darf.

## Wenn Dagger nicht startet

```
failed to select internal socket: failed to get SSH auth socket fingerprints:
failed to list SSH agent identities: agent: client error: EOF
```

Das ist kein Fehler an den Manifesten — er kommt beim Laden des Moduls. Dagger
fragt den SSH-Agent nach Identitäten, und in einer VS-Code-Remote-Sitzung zeigt
`SSH_AUTH_SOCK` gern auf einen weitergeleiteten Socket, dessen Ziel nicht mehr
existiert. Prüfen mit `ssh-add -l`; umgehen, indem man die Variable für den
Aufruf leert:

```sh
SSH_AUTH_SOCK= task kcl:kustomize
```

Nicht im Taskfile fest verdrahtet: wer SSH für private Go-Abhängigkeiten
braucht, verliert es sonst.

## Verwandt

- `docs/adr/` — Entscheidungen zu Datenmodell, Auth und Deployment
- Issue #78 — was hier entschieden wurde und warum
- Issue #89 — Phase 3 insgesamt
- `infra/cloudnative-pg` in `stuttgart-things/argocd` — Operator und Cluster
