# Deployment ohne GitOps

Der vorgesehene Weg auf einen Cluster ist eine ArgoCD-Application, die das
OCI-Artefakt `ghcr.io/stuttgart-things/schmetterpause-kustomize` konsumiert und
pro Umgebung patcht — das ist Issue #81. Dieses Dokument beschreibt den anderen
Weg: dieselben Manifeste direkt mit `kcl` und `kubectl` auf einen Cluster
bringen, ohne Argo dazwischen.

Der Weg ist gedacht für einen Testcluster, für die erste Inbetriebnahme einer
neuen Umgebung und für die Fehlersuche, wenn unklar ist, ob ein Problem von der
Anwendung oder von Argo kommt. Für den Dauerbetrieb ist er nicht gedacht: es
gibt kein Drift-Erkennung und kein Prune.

Das Beispiel verwendet durchgehend `cicd-test2`. Ersetze Clusterdomain,
Gateway-Name und Store-Name durch die der Zielumgebung.

## Was der Cluster mitbringen muss

Fünf Dinge, und jedes hat einen Befehl, der es beantwortet. Zwei davon scheitern
später stumm, wenn sie fehlen — deshalb vorher fragen und nicht hinterher raten.

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

# 4. External Secrets Operator und der Store dieses Clusters
kubectl get crd | grep externalsecrets.external-secrets.io
kubectl get clustersecretstore

# 5. Eine StorageClass für Postgres
kubectl get storageclass
```

Zu 3: Der Default in Gateway API ist `Same`. Steht dort nicht `All` oder ein
passender Selector, nimmt das Gateway die HTTPRoute aus dem Anwendungs-Namespace
nicht an — sichtbar nur an der Route, nicht am Gateway.

Zu 4: Der Store heißt `vault-<cluster>`, nie schlicht `vault`. `kubectl get
clustersecretstore` ist die verlässliche Quelle. Ein Store mit `Ready=True`
beweist, dass der Login funktioniert — nicht, dass die Policy den Eintrag lesen
darf. Das zeigt sich erst am ExternalSecret.

## Der Vault-Eintrag

Vor allem anderen, denn ohne ihn bootet die Datenbank nicht. Der Eintrag liegt
unter dem Mount, den der Store selbst trägt, und heißt `schmetterpause`.

| Schlüssel     | Inhalt                          | Form                       |
| ------------- | ------------------------------- | -------------------------- |
| `session-key` | Schlüssel für das Session-Cookie | 32 Byte, base64            |
| `username`    | Owner-Rolle der Datenbank        | `schmetterpause`           |
| `password`    | Passwort dieser Rolle            | Hex, keine Sonderzeichen   |
| `kiosk-token` | nur wenn der Kiosk an ist        | Hex                        |

`password` und `kiosk-token` sind bewusst hexadezimal und nicht base64: das
Passwort wird unmaskiert in eine DSN interpoliert, und ein `@`, `/`, `:` oder
`?` darin lässt die URL anders parsen. Der Session-Key geht durch keine URL und
darf base64 sein.

`session-key` zu ändern ist kein Neustart, sondern ein Ausloggen aller Spieler.
Der Test `TestARestartWithADifferentKeyForgetsEverybody` existiert, um genau das
festzuhalten.

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

## 3. Manifeste rendern und anwenden

```sh
kcl run kcl/main.k \
  -D config.namespace=schmetterpause \
  -D config.image=ghcr.io/stuttgart-things/schmetterpause:3382ad1 \
  -D config.clusterDomain=cicd-test2.4sthings.tiab.ssc.sva.de \
  -D config.httpRouteEnabled=true \
  -D config.gatewayName=cilium-gateway \
  -D config.gatewayNamespace=default \
  -D config.secretsMode=external \
  -D config.secretStoreKind=ClusterSecretStore \
  -D config.secretStoreName=vault-cicd-test2 \
  -D config.vaultPath=schmetterpause \
  | sed 's/^- /---\n/' | sed '1d' | sed 's/^  //' | sed '/^[[:space:]]*$/d' \
  | awk 'NR==1{print "---"} 1' | kubectl apply -f -
```

**`-D` lädt kein Profil.** `kcl/profiles/base.yaml` ist an diesem Aufruf nicht
beteiligt; jedes nicht übergebene Feld fällt auf seinen Default in `schema.k`,
nicht auf den im Profil. Die Liste oben ist deshalb vollständig zu verstehen und
nicht als Auswahl. Wer sie kürzt, bekommt stillschweigend etwas anderes — die
Defaults für `httpRouteEnabled` und `secretsMode` sind absichtlich die
zurückhaltenden.

Die `sed`-Kette ist keine Kosmetik: sie ist dieselbe Normalisierung, die
`github.com/stuttgart-things/dagger/kcl` im Publish-Pfad anwendet. Was lokal
gerendert wird, ist damit das, was auch das OCI-Artefakt trägt.

Danach zuerst die Secrets prüfen, bevor irgendetwas anderes drankommt:

```sh
kubectl -n schmetterpause get externalsecret
```

Beide müssen `SecretSynced` melden. Der Pod läuft an dieser Stelle noch nicht —
das ist richtig so, die Datenbank fehlt noch.

## 4. Postgres-Cluster

Erst jetzt, denn CloudNativePG liest beim `initdb` das Owner-Passwort aus dem
Secret, das der vorige Schritt erzeugt hat.

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
  den die DSN sucht. Er entspricht `config.dbClusterName` im KCL-Modul.
- `owner=schmetterpause` muss dem `username` im Vault-Eintrag entsprechen, sonst
  legt CNPG eine Rolle an, die die DSN nie verwendet.
- `database=schmetterpause` entspricht `config.dbName`.
- `appSecretName=schmetterpause-db` ist das vom ExternalSecret erzeugte
  `kubernetes.io/basic-auth`-Secret. Ohne diesen Wert würfelt CNPG ein eigenes
  Passwort, und die DSN passt nicht mehr dazu.

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

Die drei Fehler, die auf diesem Weg tatsächlich aufgetreten sind, haben
gemeinsam, dass sie nicht als Fehler aussehen.

**Ein ExternalSecret meldet `SecretSyncedError`, alles andere bleibt grün.**
Meist ein Property-Name, den der Vault-Eintrag nicht hat. Der fehlende Name
steht in `.status.conditions[].message`. Sichtbar ist das nur an der Ressource
selbst — Pods, Store und Gateway sagen nichts dazu.

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

## Abräumen

```sh
helmfile destroy --file 'git::https://github.com/stuttgart-things/helm.git@database/postgres-cluster.yaml.gotmpl' \
  --state-values-set namespace=schmetterpause --state-values-set clusterName=schmetterpause-db
kubectl delete ns schmetterpause
```

Das PVC des Postgres-Clusters hängt am Namespace und geht mit. Bei einer
StorageClass mit `reclaimPolicy: Delete` — etwa `openebs-hostpath` — sind die
Daten damit weg. Das ist auf einem Testcluster gewollt und anderswo nicht.
