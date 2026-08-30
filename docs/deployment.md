# Deployment ohne GitOps

Der vorgesehene Weg auf einen Cluster ist eine ArgoCD-Application, die das
OCI-Artefakt `ghcr.io/stuttgart-things/schmetterpause-kustomize` konsumiert und
pro Umgebung patcht — das ist Issue #81. Dieses Dokument beschreibt den anderen
Weg: dieselben Manifeste direkt mit `task` und `kubectl` auf einen Cluster
bringen, ohne Argo dazwischen.

Gedacht für einen Testcluster, für die erste Inbetriebnahme einer neuen Umgebung
und für die Fehlersuche, wenn unklar ist, ob ein Problem von der Anwendung oder
von Argo kommt. Für den Dauerbetrieb nicht: es gibt keine Drift-Erkennung und
kein Prune.

Das Beispiel verwendet durchgehend `cicd-test2`. Ersetze Clusterdomain,
Gateway-Name und Store-Name durch die der Zielumgebung.

## Was der Cluster mitbringen muss

Drei Dinge, plus eines, das von der gewählten Secrets-Variante abhängt.

```sh
# 1. CloudNativePG-Operator
kubectl -n postgres get deploy cloudnative-pg

# 2. Gateway, programmiert, mit einem TLS-Listener
kubectl -n default get gateway cilium-gateway
kubectl -n default get gateway cilium-gateway \
  -o jsonpath='{range .spec.listeners[*]}{.name}  {.hostname}  {.tls.certificateRefs[*].name}{"\n"}{end}'

# 3. Nimmt das Gateway Routes aus fremden Namespaces?
kubectl -n default get gateway cilium-gateway \
  -o jsonpath='{range .spec.listeners[*]}{.name}  {.allowedRoutes.namespaces.from}{"\n"}{end}'

# 4. Eine StorageClass für Postgres
kubectl get storageclass
```

Zu 3: Der Default in Gateway API ist `Same`. Steht dort nicht `All` oder ein
passender Selector, nimmt das Gateway die HTTPRoute aus dem Anwendungs-Namespace
nicht an — sichtbar nur an der Route, nicht am Gateway.

**Der External Secrets Operator ist keine Voraussetzung.** Er ist der bequemere
von zwei Wegen, nicht der einzige; der nächste Abschnitt stellt beide
nebeneinander.

## Secrets: zwei Wege

Die Anwendung braucht zwei Secrets, und beide heißen unter jeder Variante
gleich, weil das Deployment sie über den Namen liest:

| Secret                | Schlüssel                          | Wer liest das                       |
| --------------------- | ---------------------------------- | ----------------------------------- |
| `schmetterpause-app`  | `SP_SESSION_KEY`, ggf. `SP_KIOSK_TOKEN` | die Anwendung                   |
| `schmetterpause-db`   | `username`, `password`, `SP_DATABASE_URL` | CloudNativePG und die Anwendung |

`schmetterpause-db` ist vom Typ `kubernetes.io/basic-auth` und tut doppelten
Dienst: CloudNativePG liest daraus beim `initdb` die Owner-Credentials, die
Anwendung liest `SP_DATABASE_URL`. Dass beide aus demselben Secret kommen, ist
der Grund, warum Rolle und DSN nicht auseinanderlaufen können.

Zwei Secrets statt einem ist Least Privilege: der Migrations-initContainer
bekommt nur das Datenbank-Secret und sieht den Session-Key nie.

`SP_SESSION_KEY` zu ändern ist kein Neustart, sondern ein Ausloggen aller
Spieler. Der Test `TestARestartWithADifferentKeyForgetsEverybody` hält das fest.

### Variante A — External Secrets

Der Weg für einen Cluster, der ohnehin einen `ClusterSecretStore` hat. Die
beiden Secrets werden dann aus einem Vault-Eintrag erzeugt und turnusmäßig
erneuert; im Repo und in der Kommandozeile steht nur ein Pfad, nie ein Wert.

```sh
kubectl get crd | grep externalsecrets.external-secrets.io
kubectl get clustersecretstore
```

Der Store heißt `vault-<cluster>`, nie schlicht `vault`. `kubectl get
clustersecretstore` ist die verlässliche Quelle. Ein Store mit `Ready=True`
beweist, dass der Login funktioniert — nicht, dass die Policy den Eintrag lesen
darf. Das zeigt sich erst am ExternalSecret.

Der Eintrag liegt unter dem Mount, den der Store selbst trägt, und heißt
`schmetterpause`:

| Schlüssel     | Inhalt                           | Form                     |
| ------------- | -------------------------------- | ------------------------ |
| `session-key` | Schlüssel für das Session-Cookie | 32 Byte, base64          |
| `username`    | Owner-Rolle der Datenbank        | `schmetterpause`         |
| `password`    | Passwort dieser Rolle            | Hex, keine Sonderzeichen |
| `kiosk-token` | nur wenn der Kiosk an ist        | Hex                      |

`password` und `kiosk-token` sind bewusst hexadezimal und nicht base64: das
Passwort wird unmaskiert in eine DSN interpoliert, und ein `@`, `/`, `:` oder
`?` darin lässt die URL anders parsen. Der Session-Key geht durch keine URL und
darf base64 sein.

Im `remoteRef.key` steht nur der Eintragsname. Der Store trägt Mount und
`version: v2` selbst, ESO setzt daraus `<mount>/data/<key>` zusammen. Beide Arten,
das falsch zu machen, zeigen auf etwas, das nirgends existiert, und melden beim
Apply nichts:

```
schmetterpause                    richtig
schmetterpause/data/cicd-test2    -> cicd-test2/data/schmetterpause/data/…
schmetterpause-cicd-test2         -> der Cluster zweimal; der Mount IST der Cluster
```

### Variante B — normale Kubernetes-Secrets

Der Weg für einen Cluster ohne ESO. Ein Befehl, der beide Secrets erzeugt, die
Werte würfelt und die DSN so zusammensetzt, wie das ESO-Template es täte:

```sh
kubectl create ns schmetterpause
task kcl:secrets NAMESPACE=schmetterpause
```

Der Task legt an und aktualisiert nicht. Ein zweiter Aufruf bricht ab, statt
einen neuen Session-Key zu setzen — das wäre kein Fehler, den man sieht, sondern
einer, bei dem am nächsten Morgen alle ausgeloggt sind.

Mit Kiosk: `task kcl:secrets NAMESPACE=schmetterpause KIOSK=true`.

Von Hand geht es genauso, wenn die Werte schon anderswo herkommen — wichtig ist
nur, dass das Passwort in `SP_DATABASE_URL` dasselbe ist wie unter `password`
und keine Zeichen enthält, die eine URL zerlegen.

Ab hier unterscheidet sich nur noch das Profil: **Variante A rendert mit
`PROFILE=base`, Variante B mit `PROFILE=existing-secrets`.**

## 1. Postgres-Operator

Einmal pro Cluster, nicht pro Anwendung:

```sh
helmfile apply \
  --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres.yaml.gotmpl' \
  --state-values-set namespace=postgres \
  --state-values-set version=0.29.0
```

## 2. Namespace

Die Base rendert bewusst keinen. Mehrere Anwendungen können sich einen Namespace
teilen, und wenn mehr als eine ihn als Ressource mitbringt, meldet ArgoCD eine
SharedResource — dann löscht der Prune-Lauf der einen den Namespace der anderen
weg.

```sh
kubectl create ns schmetterpause
```

## 3. Manifeste anwenden

```sh
task kcl:apply -- \
  -D config.image=ghcr.io/stuttgart-things/schmetterpause:3382ad1 \
  -D config.clusterDomain=cicd-test2.4sthings.tiab.ssc.sva.de \
  -D config.gatewayName=cilium-gateway \
  -D config.secretStoreName=vault-cicd-test2 \
  -D config.vaultPath=schmetterpause
```

Für Variante B ohne die beiden letzten Zeilen und mit dem anderen Profil:

```sh
task kcl:apply PROFILE=existing-secrets -- \
  -D config.image=ghcr.io/stuttgart-things/schmetterpause:3382ad1 \
  -D config.clusterDomain=cicd-test2.4sthings.tiab.ssc.sva.de \
  -D config.gatewayName=cilium-gateway
```

Alles nach `--` wird an `kcl` durchgereicht, hinter die Werte des Profils, und
ein späteres `-D` gewinnt. Das Profil liefert also die Grundlage, die
Kommandozeile die Handvoll Werte, die diese Umgebung ausmachen.

**Ohne Profil rendern ist die Falle.** `kcl run kcl/main.k -D …` lädt kein
Profil; jedes nicht übergebene Feld fällt dann auf seinen Default in `schema.k`,
nicht auf den im Profil. `httpRouteEnabled` ist dort `false` und `secretsMode`
ist `external` — beides absichtlich zurückhaltend, und beides nicht das, was ein
Cluster will. Deshalb `task kcl:apply` und nicht `kcl run`.

`task kcl:render` statt `kcl:apply` zeigt dasselbe, ohne es anzuwenden.

Bei Variante A danach zuerst die Secrets prüfen, bevor irgendetwas anderes
drankommt:

```sh
kubectl -n schmetterpause get externalsecret
```

Beide müssen `SecretSynced` melden. Der Pod läuft an dieser Stelle noch nicht —
das ist richtig so, die Datenbank fehlt noch.

## 4. Postgres-Cluster

Erst jetzt, denn CloudNativePG liest beim `initdb` das Owner-Passwort aus
`schmetterpause-db`, und das muss vorher stehen — in Variante A, sobald das
ExternalSecret synchronisiert hat, in Variante B seit `task kcl:secrets`.

```sh
helmfile apply \
  --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres-cluster.yaml.gotmpl' \
  --state-values-set namespace=schmetterpause \
  --state-values-set version=0.8.1 \
  --state-values-set clusterName=schmetterpause-db \
  --state-values-set database=schmetterpause \
  --state-values-set owner=schmetterpause \
  --state-values-set appSecretName=schmetterpause-db
```

Die vier fachlichen Werte hängen aneinander:

- `clusterName=schmetterpause-db` erzeugt den Service `schmetterpause-db-rw`,
  den die DSN sucht. Entspricht `config.dbClusterName` im KCL-Modul.
- `owner=schmetterpause` muss dem `username` im Secret entsprechen, sonst legt
  CNPG eine Rolle an, die die DSN nie verwendet.
- `database=schmetterpause` entspricht `config.dbName`.
- `appSecretName=schmetterpause-db` ist das oben erzeugte Secret. Ohne diesen
  Wert würfelt CNPG ein eigenes Passwort, und die DSN passt nicht mehr dazu.

Alles Übrige kommt aus den Defaults des Templates: eine Instanz, PostgreSQL 17,
8Gi, die Default-StorageClass, keine Backups, kein Superuser-Zugang. Warum eine
Instanz und keine Backups: Replikate und Backups schützen gegen verschiedene
Dinge, und die realistische Gefahr auf einem Testcluster ist ein bewusster
Neubau, gegen den ein Replikat nichts ausrichtet. Der Auslöser zum Umstellen
sind Backups, nicht Replikate.

Das PVC entsteht erst mit dem Pod, wenn die StorageClass
`WaitForFirstConsumer` ist.

Sobald der Cluster steht, läuft der Migrations-initContainer beim nächsten
Backoff von selbst durch. Nachhelfen muss man nicht; wer nicht warten will,
löscht den Pod.

## 5. Prüfen

```sh
kubectl -n schmetterpause get cluster
kubectl -n schmetterpause get po
kubectl -n schmetterpause get httproute schmetterpause -o yaml | sed -n '/^status:/,$p'

curl -sSI https://schmetterpause.cicd-test2.4sthings.tiab.ssc.sva.de/healthz   # 200
curl -sSI http://schmetterpause.cicd-test2.4sthings.tiab.ssc.sva.de/           # 301
```

## Wenn etwas nicht geht

Die Fehler, die auf diesem Weg tatsächlich aufgetreten sind, haben gemeinsam,
dass sie nicht als Fehler aussehen.

**Ein ExternalSecret meldet `SecretSyncedError`, alles andere bleibt grün.**
Nur Variante A. Meist ein Property-Name, den der Vault-Eintrag nicht hat. Der
fehlende Name steht in `.status.conditions[].message`. Sichtbar ist das nur an
der Ressource selbst — Pods, Store und Gateway sagen nichts dazu.

**Die HTTPRoute hat gar keinen Status, und der Host antwortet 404.**
Nicht `Accepted=False`, sondern ein leeres `.status.parents`. Dann zeigt der
`parentRef` auf ein Gateway, das es nicht gibt — typischerweise, weil
`namespace` fehlt und deshalb der Namespace der Route selbst gemeint ist.
`schema.k` weist einen leeren `gatewayNamespace` inzwischen ab; wenn die Route
trotzdem statuslos ist, stimmen Name oder `sectionName` nicht.

**Der initContainer scheitert mit `no such host`.** Die Datenbank fehlt noch
oder heißt anders. Die Meldung ist trotzdem nützlich: steht darin
`user=schmetterpause database=schmetterpause`, ist die DSN sauber geparst, das
Passwort enthält also kein Zeichen, das die URL zerlegt, und es fehlt wirklich
nur der Name.

**Der Pod startet nicht, `secret "schmetterpause-db" not found`.** Die
Reihenfolge stimmt nicht: das Secret muss vor dem Deployment da sein. In
Variante A ist das eine Frage von Sekunden und löst sich selbst, in Variante B
heißt es, `task kcl:secrets` wurde übersprungen.

## Abräumen

```sh
helmfile destroy --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres-cluster.yaml.gotmpl' \
  --state-values-set namespace=schmetterpause --state-values-set clusterName=schmetterpause-db
kubectl delete ns schmetterpause
```

Das PVC des Postgres-Clusters hängt am Namespace und geht mit. Bei einer
StorageClass mit `reclaimPolicy: Delete` — etwa `openebs-hostpath` — sind die
Daten damit weg. Das ist auf einem Testcluster gewollt und anderswo nicht.
