# ADR: Deployment-Ziel Azure Local / Kubernetes (Entwurf)

- **Status:** Entwurf — noch nicht mit dem Team abgestimmt, kein `accepted`
- **Nummer:** noch nicht vergeben. Wird beim Merge zugeteilt; bis dahin trägt
  der Dateiname `XXXX`. Grund: Auf `main` entstehen parallel ADRs, und zwei
  gleich nummerierte Dateien mit verschiedenen Namen mergen konfliktfrei —
  Git meldet das nicht.
- **Datum:** 2026-08-21
- **Betrifft:** Deployment, Infrastruktur
- **Phase:** phase-3, entsprechend #89. Die ursprüngliche Einordnung als
  phase-2 mitsamt Sperre bis zum Ende der MVP-Messung war zu weit gefasst:
  #89 hält Phase 3 ausdrücklich unabhängig von der Messung — gebunden ist
  allein, dass die Office-Compose-Installation unangetastet weiterläuft.
  Planen und Schreiben waren nie das Problem, das Anfassen der laufenden
  Installation ist es.
- **Verwandt:** das Backup-ADR aus demselben Branch, ADR-0002 (kein Redis)

## Kontext

`docs/mvp-plan.md` schließt Kubernetes- und ACA-Deployment bewusst aus dem
MVP-Scope aus. Die Invarianten aus `CLAUDE.md` (ein Image, Konfiguration
ausschließlich über Env-Variablen, kein Zustand im Container) gelten trotzdem
ab der ersten Zeile — der Weg dorthin wird nicht verbaut.

Dieses ADR bereitet vor, *worauf* dieser Weg zuläuft, ohne den MVP-Zeitplan zu
berühren. Es entsteht parallel zu AP3–AP7, nicht davor.

Zielumgebung ist Azure Local. Ein Azure-Local-Cluster registriert sich als
Azure-Arc-enabled-Kubernetes-Cluster — die Deployment-Frage ist damit auch
eine Arc-Frage.

Dieses ADR behandelt, *wohin* deployt wird. Wie der Zustand einen
Cluster-Neubau übersteht, ist eine eigene Entscheidung mit eigenem
Lebenszyklus und steht im Backup-ADR.

## Zielumgebung Azure Local, Übergang auf dem bestehenden Cluster

Aus der Statusbesprechung vom 2026-08-28: **Azure Local ist das Ziel in der
finalen Konfiguration. Steht die Azure-Local-Seite nicht rechtzeitig bereit,
wird übergangsweise auf den bestehenden stuttgart-things-Cluster deployt.**

Das ist die Information, die #89 fehlt, und sie erklärt, warum dort weder
Azure Local noch Arc vorkommen: #78, #81 und #82 beschreiben durchgehend eine
Umgebung, in der Gateway, Argo CD, Vault und Velero bereits laufen —
`sthings-gateway` in `ingress-system`, `stuttgart-things/argocd`,
`infra/velero`. Für den Übergang trifft das zu. Für die Zielumgebung ist jede
dieser Voraussetzungen ein eigenes Stück Arbeit, das in Phase 3 bisher
nirgends steht.

Zwei Dinge folgen daraus:

- **Die Manifestform darf nicht an der Umgebung hängen.** Invariante 1 fordert
  ein Image für alle Ziele; dasselbe muss für die kustomize-Base gelten. Sonst
  hat der Übergang eine andere Wahrheit als das Ziel, und der Wechsel dorthin
  wird eine zweite Migration statt eines Umzugs. Wo Azure Local etwas nicht
  mitbringt, wird es **dort nachgerüstet** — nicht die Base umgeschrieben.
- **Zwei Umgebungen sind zwei Stages.** Läuft Phase 3 erst auf dem
  sthings-Cluster und kommt Azure Local danach dazu, ist der erste Auslöser
  aus #83 erfüllt: zwei Umgebungen mit einer echten Entscheidung dazwischen.
  Das ist kein Grund, Kargo jetzt zu bauen. Aber #83 nimmt an, dieser Fall sei
  fern, und das stimmt dann nicht mehr.

## Was Azure Local nicht mitbringt

AKS enabled by Azure Arc auf Azure Local ist Kubernetes, aber nicht dasselbe
Kubernetes, gegen das Phase 3 gebaut wurde. Die linke Spalte war bis zum
2026-08-28 geraten; seit den Erhebungen in #78 sind es **gelesene Werte vom
Cluster `cicd-test2`**. Das macht die Tabelle erst brauchbar: Sie vergleicht
jetzt zwei konkrete Umgebungen statt einer konkreten mit einer vermuteten.

| Baustein | `cicd-test2` (gelesen, #78) | AKS enabled by Azure Arc | Folge |
| --- | --- | --- | --- |
| Arc-Registrierung | nicht relevant | AKS auf Azure Local ist **ab Werk Arc-registriert** | Entlastung. Der Kontext oben formuliert das noch als eigenen Vorgang; das ist er nicht. |
| Gateway | `cilium-gateway` in `default`, Klasse `cilium`, Adresse `10.100.136.227`. Zwei Listener auf demselben Wildcard: `https` mit `wildcard-cicd-test2-tls`, `http` ohne Redirect | Gateway API ist **nicht vorinstalliert**; dokumentiert ist der NGINX Ingress Controller. Ein Controller muss mitgebracht werden | **Größter Posten.** Und er ist größer als gedacht: Die Base bindet über `sectionName` an Listener namens `https` und `http` (#78). Ein Azure-Local-Gateway muss diese Namen tragen, sonst greift der Redirect-Trick nicht — oder das Profil überschreibt `gatewaySectionNameHTTPS`/`…HTTP`. |
| Zertifikat | Wildcard `*.cicd-test2.4sthings.tiab.ssc.sva.de` am Listener — keine Arbeit auf unserer Seite | Kein Gateway, also kein Listener, also kein Zertifikat | cert-manager oder ein vorhandenes Wildcard. Das ist der Teil von #74, der auf Azure Local **nicht** verschwindet. |
| Externe IPs | am Gateway vorhanden | **MetalLB** als Arc-Extension oder eigener Loadbalancer. Der IP-Bereich darf nicht mit Arc-VM-Logical-Networks oder Control-Plane-IPs kollidieren | Ein IP-Bereich muss reserviert und dokumentiert sein, bevor überhaupt etwas erreichbar ist. |
| GitOps-Installation | Argo CD läuft, gepflegt in `stuttgart-things/argocd` | Zwei Wege: Arc-Extension `microsoft.argocd` (**Public Preview**) oder Argo selbst per Helm. Die Flux-Extension ist GA — aber #81 hat Argo entschieden | Eigene Entscheidung, siehe offene Frage 6. |
| Secret-Store | `ClusterSecretStore` `vault-cicd-test2` gegen OpenBao, Muster `vault-<cluster>` aus dem Backstage-Template. Einträge SOPS-verschlüsselt per Terraform in `stuttgart-things/argocd` | Existiert dort nicht. Alternativen: dasselbe Muster nachbauen, oder **Azure Key Vault mit Workload Identity** | Beides ist ein `secretStoreName` im Profil — die Base ist neutral (#78, `secretsMode`). Die Arbeit liegt cluster-seitig, nicht in den Manifesten. |
| StorageClass | `openebs-hostpath` (Default), `openebs.io/local`, node-lokal, `Delete`-Reclaim, keine Volume-Expansion | `disk.csi.akshci.com`, VHDX-gestützt. **Linux-Workloads brauchen eine eigene StorageClass mit `fsType: ext4`** — die Default genügt nicht | Die `Cluster`-Ressource kommt aus dem Katalog `infra/cloudnative-pg`, gehört also der Argo-Application und nicht unserer Base — die `storageClass` ist dort zu setzen. |
| Volume-Snapshots | **Ungeprüft.** `openebs-hostpath` ist ein LocalPV-Provisioner; ob eine `VolumeSnapshotClass` existiert, beantwortet `kubectl get volumesnapshotclass` | **Werden nicht unterstützt** | Betrifft #84: Wenn *beide* Umgebungen keine Snapshots haben, ist Weg A dort überall Datei-Backup, und der Vergleich hat ein Kriterium weniger. Gehört geprüft, bevor er aufgesetzt wird. |
| Objektspeicher | `infra/velero` gegen S3-kompatiblen Speicher, Zugang als `cloud-credentials`-ExternalSecret aus Vault | Azure Local bringt **keinen S3-Dienst** mit. Ziel wäre Azure Blob Storage in Azure | Die Randbedingung „außerhalb des Clusters" aus dem Backup-ADR erfüllt sich von selbst. Dafür hängt das Backup am WAN-Link. |

Belege: [CSI-Disk-Treiber in AKS
Arc](https://learn.microsoft.com/en-us/azure/aks/aksarc/container-storage-interface-disks),
[Backup mit Velero — keine
Volume-Snapshots](https://learn.microsoft.com/en-us/azure/aks/aksarc/backup-workload-cluster),
[MetalLB-Übersicht](https://learn.microsoft.com/en-us/azure/aks/aksarc/load-balancer-overview),
[Ingress in AKS
Arc](https://learn.microsoft.com/en-us/azure/aks/aksarc/create-ingress-controller),
[Argo-CD-Extension für
Arc](https://learn.microsoft.com/en-us/azure/azure-arc/kubernetes/conceptual-gitops-argocd).

## Offene Fragen (noch nicht entschieden)

1. ~~**GitOps-Mechanismus**~~ — **entschieden** (Issue #81, 2026-08-26):
   Argo CD. Flux wird für diese Anwendung nicht verfolgt.

   Die Begründung ist eine andere als die hier ursprünglich vermutete. Nicht
   Kargo gab den Ausschlag, sondern der `pullRequest`-Generator eines
   `ApplicationSet`: Die Vorschau-Umgebung je Pull Request (#82) hängt daran,
   und Flux hat dafür kein Gegenstück. Kargo ist inzwischen ein eigenes Issue
   (#83, phase-4) mit vier Auslösern, von denen keiner erfüllt ist — dasselbe
   Muster wie ADR-0002.

   Offen bleibt die Frage, die #81 nicht stellt, weil sie sich dort nicht
   stellt: *wie* Argo CD auf Azure Local dorthin kommt. Siehe den Abschnitt
   zur Zielumgebung.
2. ~~**Wie viele Stages?**~~ — beantwortet, mit einem Vorbehalt: Phase 3 baut
   genau eine. Der Vorbehalt steht im Abschnitt zur Zielumgebung — trägt der
   sthings-Cluster den Übergang und kommt Azure Local danach dazu, sind es
   zwei, und der erste Kargo-Auslöser aus #83 ist erfüllt, bevor jemand ihn
   geprüft hat.
3. ~~**Registry**~~ — **entschieden** (Issue #20, 2026-08-21): `ttl.sh` für
   Wegwerf-Builds, `ghcr.io/stuttgart-things/schmetterpause` für alles
   Bleibende. Multi-Arch (amd64+arm64), Push bei jedem `main`-Merge als
   Snapshot, `latest` bewegt sich nur bei echten Tags (`dagger/main.go`,
   `Release`-Funktion). Für dieses ADR heißt das: Die Arc/Argo-CD-Extension
   zieht Images von GHCR — bei privatem Package braucht der Azure-Local-
   Cluster ein `imagePullSecret`, bei öffentlichem nicht. Muss noch geprüft
   werden, wie das GHCR-Package aktuell sichtbar ist.
4. ~~**Skalierung / Redis-Trigger**~~ — **entschieden** (Issue #78): Es bleibt
   bei `replicas: 1` mit `strategy: Recreate`.

   Ausschlaggebend ist nicht die Last, sondern die Migration.
   `postgres.Migrate` ruft `goose.UpContext` über die Paket-API auf, und die
   nimmt keinen Session-Lock — den gibt es nur auf goose' Provider-API. Zwei
   gleichzeitig migrierende Replicas sind damit tatsächlich unsicher, nicht
   theoretisch. Mehr als eine Replica ist ausdrücklich eigene Arbeit
   (Advisory-Lock um die Migration, oder ein Job vor dem Rollout).

   Damit ist der Auslöser aus ADR-0002 nicht erfüllt, und ADR-0002 bleibt
   unangetastet. Das ist eine Antwort, kein Aufschub.
5. **Secret-Verwaltung** — kleiner geworden, aber noch offen. Die Frage lautete
   allgemein: Wie kommen Zugangsdaten in einen frisch gebauten Cluster, bevor
   Argo CD läuft? Drei Fälle fallen inzwischen auseinander:

   - **`SP_DATABASE_URL` löst sich *nicht* von selbst** — hier stand, CNPG
     erzeuge ein `<cluster>-app`-Secret mit fertiger Connection-URI, und das
     war aus dem ursprünglichen #78-Text übernommen. **Es trägt nicht:** Das
     Katalog-README dokumentiert `username`, `password` und `dbname`, aber
     keinen fertigen `uri`-Key, und unsere Konfiguration kann aus Teilen keine
     DSN zusammensetzen — `SP_DATABASE_URL` ist genau ein String.

     Gebaut wurde deshalb der umgekehrte Weg: **ESO besitzt das Secret**, CNPG
     übernimmt es beim Bootstrap als Eigentümer-Zugang statt selbst eines zu
     erzeugen, und das ESO-`template` setzt die URL zusammen —
     `postgresql://<owner>@<clusterName>-rw.<ns>.svc:5432/<database>`. Vault
     hält nur das Passwort, die Topologie bleibt in den Manifesten.
   - **Der Objektspeicher-Zugang löst sich auf Azure Local von selbst.** Das
     barman-cloud-Plugin kann sich über `inheritFromAzureAD` beziehungsweise
     die Default-Credential-Kette gegen Azure Blob Storage authentifizieren —
     **ohne gespeichertes Geheimnis**. Das ist der Fall, den das Backup-ADR
     als „der Punkt, der bei einer echten Wiederherstellung tatsächlich
     schmerzt" benennt, und auf der Zielumgebung schmerzt er weniger als auf
     der Übergangsumgebung. Ein Argument für Azure Local, das dort noch fehlt.
   - **`SP_SESSION_KEY` und `SP_KIOSK_TOKEN` sind entschieden** (#78,
     2026-08-29): **External Secrets**, und die KCL rendert den
     `ExternalSecret` selbst. Der hier vorgeschlagene „Schritt null von Hand"
     ist nicht verworfen, sondern eingebaut — als Profil `existing-secrets`
     und als `task kcl:secrets`, für Cluster ohne ESO. Genau der Fall, der auf
     Azure Local zuerst eintreten dürfte.

     Bemerkenswert ist, wie die Regel dabei *strenger* geworden ist statt
     lockerer: Das Schema nimmt weiterhin keinen Geheimniswert an — es nimmt
     einen **Vault-Pfad**. Ein falscher Pfad scheitert laut beim Sync; ein
     falscher Wert hätte still das ganze Büro ausgeloggt. Und die zwei
     Secrets sind getrennt (`schmetterpause-db`, `schmetterpause-app`), damit
     der Migrations-initContainer den Cookie-Schlüssel nie sieht.

     **Für Azure Local bleibt genau eine Frage übrig:** woher der Store kommt.
     Das Muster `vault-<cluster>` stammt aus einem Backstage-Template, das es
     dort nicht gibt. Entweder nachbauen, oder Azure Key Vault mit Workload
     Identity — was zum Managed-Identity-Weg beim Objektspeicher passen würde,
     also ein Mechanismus statt zwei.

   `imagePullSecret` aus Punkt 3 hängt weiterhin daran, ob das GHCR-Package
   öffentlich ist. Ungeprüft.

6. **Wie kommt Argo CD auf Azure Local?** #81 stellt die Frage nicht, weil dort
   bereits eine Instanz läuft. Auf der Zielumgebung gibt es zwei Wege:

   | | Dafür | Dagegen |
   | --- | --- | --- |
   | **Arc-Extension** `microsoft.argocd` | Portal-Integration, Workload Identity Federation gegen ACR und Azure DevOps — also wieder Zugang ohne Langzeit-Credentials, wie bei Punkt 5 | **Public Preview**, nicht GA. Ein Preview-Dienst im Pfad jedes Deploys |
   | **Argo selbst per Helm** | Kein Preview-Risiko, identisch mit dem sthings-Cluster, ein Betriebsmodell statt zwei | Die Arc-Vorteile fallen weg, Betrieb liegt bei uns |

   Schwer rückabzuwickeln, deshalb hier und nicht nebenbei. Betrifft nur die
   Zielumgebung — der Übergang ist davon unberührt.
7. **Hostname und DNS** — für die Übergangsumgebung beantwortet, für die
   Zielumgebung offen, und dazwischen liegt das eigentliche Risiko.

   Auf `cicd-test2` ist es entschieden (#78): `schmetterpause` als Label,
   zusammengesetzt mit der Cluster-Domain, gedeckt vom Wildcard-Zertifikat am
   Listener. #74 ist damit genau das geworden, was vorhergesagt war — ein Name
   und `SP_PUBLIC_BASE_URL`.

   **Der Wechsel auf Azure Local ändert diesen Namen.** ADR-0004 legt uns auf
   WebAuthn fest, und Passkeys hängen an der Relying-Party-ID, also am
   Hostnamen. Ein Umzug entwertet sie — auch bei fehlerfreiem
   Datenbank-Restore. Zwei Dinge entschärfen das, keines löst es:

   - **WebAuthn ist noch nicht ausgeliefert.** Solange #37 offen ist, gibt es
     keine Passkeys, die kaputtgehen könnten. Das Zeitfenster ist also: der
     Umzug muss *vor* WebAuthn passieren, oder der Name muss ihn überleben.
   - **Wiederherstellungscode und PIN sind seit dem 2026-08-28 im Code**
     (#98, #100, #101, #103). Sie hängen nicht am Hostnamen, ein Spieler kommt
     nach einem Umzug also wieder an seine Zeile. Das macht den Umzug
     unbequem statt teuer — für alles außer Passkeys.

   Zu klären bleibt, wer die Zone besitzt und ob ein Name über beide
   Umgebungen tragen kann. Das ist keine Frage an uns.

## Hinweis zur Form

Das Team trackt offene Punkte als GitHub-Issues statt als ADR-Prosa — ADRs für
Entscheidungen mit Bestand, Issues für Fragen, die noch offen sind. Genau das
ist mit diesem Thema passiert: Aus dem, was hier als Unterpunkte stand, sind
#74 und #78 bis #86 geworden, gebündelt unter #89.

Für die Punkte, die oben offen bleiben, gilt derselbe Schnitt. Sie gehören als
Kommentar an die bestehenden Issues, nicht als neue: Punkt 5 und 6 an #78 und
#81, wo dieselben Checkboxen schon stehen — nur für eine andere Umgebung
beantwortet. Punkt 7 ist #74.

## Entscheidung

Die Deployment-Frage selbst ist inzwischen woanders entschieden: Kubernetes
statt Azure Container Apps, KCL für die Manifeste, Auslieferung als
kustomize-OCI-Artefakt, Argo CD als Reconciliation-Schicht, CloudNativePG für
Postgres (#78, #80, #81). Dieses ADR schreibt das nicht neu.

*Was hier zur Entscheidung steht, ist der Teil, den Phase 3 nicht behandelt:*

1. **Azure Local ist die Zielumgebung, der sthings-Cluster der Übergang** —
   nicht umgekehrt, und der Übergang ist kein Zwischenstand, den man später
   stehen lässt.
2. **Was Azure Local nicht mitbringt, wird dort nachgerüstet** — Gateway-API-
   Controller, Loadbalancer, StorageClass, Secret-Store. Die kustomize-Base
   bleibt für beide Umgebungen dieselbe. Das ist Invariante 1, eine Ebene höher
   gezogen.

   **Dieser Punkt ist inzwischen keine Forderung mehr, sondern eine
   Eigenschaft.** `kcl/schema.k` beginnt mit „Everything is a variable" —
   Hostname, Namespace, Gateway, Cluster-Domain, Image und Secret-Store sind
   getypte Felder ohne eingebackene Werte, und `existing-secrets` deckt den
   Fall ohne ESO ab. Azure Local braucht damit **kein zweites Manifest-Set,
   sondern ein Profil**. Das ist die wichtigste Änderung an diesem ADR seit
   seiner ersten Fassung, und sie kam nicht von uns.
3. **Der Objektspeicher-Zugang läuft über Managed Identity**, nicht über ein
   Geheimnis in Git. Damit bleibt als Bootstrap-Problem nur, was die Anwendung
   selbst braucht, und das sind zwei Werte. Auf der Übergangsumgebung ist
   derselbe Fall anders gelöst — ESO gegen OpenBao, Einträge per Terraform —
   und beides nebeneinander ist in Ordnung: Der Store ist ein Profilwert.

Offen und nicht allein entscheidbar: der Hostname (Punkt 7), die Art, wie Argo
CD auf Azure Local installiert wird (Punkt 6), und woher dort der Secret-Store
kommt (Punkt 5).

## Was Azure Local konkret braucht

Weil die Base neutral ist, zerfällt der verbleibende Teil in zwei Listen. Die
erste ist eine Datei, die zweite ist Cluster-Arbeit — und nur die zweite ist
aufwendig.

**Das Profil** (`kcl/profiles/`, nach dem Muster von `base.yaml`):

- `config.clusterDomain` — die Azure-Local-Cluster-Domain
- `config.gatewayName` / `config.gatewayNamespace` — was dort installiert wird
- `config.gatewaySectionNameHTTPS` / `…HTTP` — nur falls die Listener anders
  heißen als `https`/`http`
- `config.secretStoreName` + `config.vaultPath`, oder `config.secretsMode:
  existing` für den Anfang
- `config.image` — unverändert, dasselbe Image (Invariante 1)

**Der Cluster** — das ist die Arbeit:

- [ ] Gateway-API-Controller installieren, mit Listenern `https` und `http`
- [ ] Zertifikat für die Cluster-Domain, am Listener
- [ ] MetalLB oder ein anderer Loadbalancer, mit reserviertem IP-Bereich
- [ ] StorageClass mit `fsType: ext4` für die CNPG-Instanz
- [ ] Argo CD — Arc-Extension oder selbst betrieben (Punkt 6)
- [ ] Secret-Store, oder bewusst `existing-secrets` und ein Schritt null
- [ ] CloudNativePG-Operator aus dem Katalog `infra/cloudnative-pg`

Nichts davon berührt das Anwendungs-Repository.

## Nicht Teil dieses ADR

Das Whiteboard-Vorhaben "TT-Zählwerk" (Kamera/Piezo/ESP32/iPad zur
automatischen Ergebniserfassung) ist ein separates Hardware-Thema, keine
Deployment-Frage der App. Zu klären, bevor es weiterverfolgt wird: Es steht in
Spannung zur MVP-Definition-of-Done, die *freiwilliges manuelles* Eintragen
misst — eine Automatisierung der Erfassung würde genau diese Messung
verändern.

## Konsequenzen

- **Positiv:** Entscheidung liegt vor, sobald sie gebraucht wird, statt unter
  Zeitdruck am Ende des MVP.
- **Risiko:** Als Entwurf kann sich das mit echten Constraints aus Azure Local
  (Netzwerk, verfügbare Extensions, Node-Zahl) noch ändern — bewusst nicht
  vorschnell auf `accepted` gesetzt.
