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
Kubernetes, gegen das Phase 3 geschrieben ist. Sieben Unterschiede, die vor
dem ersten Deploy dorthin geklärt sein müssen — Stand der
Microsoft-Dokumentation am 2026-08-28:

| Baustein | Im sthings-Cluster | AKS enabled by Azure Arc | Folge |
| --- | --- | --- | --- |
| Arc-Registrierung | nicht relevant | AKS auf Azure Local ist **ab Werk Arc-registriert** | Entlastung. Der Kontext oben formuliert das noch als eigenen Vorgang; das ist er nicht. |
| Ingress | Cilium Gateway API, Zertifikat am Listener des Gateways | Dokumentierter Weg ist der **NGINX Ingress Controller**. Ein Gateway-API-Controller ist nicht vorinstalliert | **Größter Posten.** `httproute.k` aus #78 setzt eine Gateway-API-Implementierung voraus. Nach der Regel oben wird sie auf Azure Local nachgerüstet. |
| Externe IPs | vorhanden | **MetalLB** als Arc-Extension oder ein eigener Loadbalancer. Der IP-Bereich darf nicht mit Arc-VM-Logical-Networks oder Control-Plane-IPs kollidieren | Ein IP-Bereich muss reserviert und dokumentiert sein, bevor überhaupt etwas erreichbar ist. |
| GitOps-Installation | Argo CD läuft, gepflegt in `stuttgart-things/argocd` | Zwei Wege: Arc-Extension `microsoft.argocd` (**Public Preview**) oder Argo selbst per Helm. Die Flux-Extension ist GA — aber #81 hat Argo entschieden | Eigene Entscheidung, siehe offene Frage 6. |
| StorageClass | vorhanden | `disk.csi.akshci.com`, VHDX-gestützt. **Linux-Workloads brauchen eine eigene StorageClass mit `fsType: ext4`** — die Default genügt nicht | Die `Cluster`-Ressource kommt aus dem Katalog `infra/cloudnative-pg`, gehört also der Argo-Application und nicht unserer Base — die `storageClass` ist dort zu setzen. Auf `cicd-test2` ist die Default `openebs-hostpath`; auf Azure Local gibt es kein Gegenstück, das ohne `fsType` funktioniert. |
| Volume-Snapshots | vorhanden | **werden nicht unterstützt** | Betrifft das Backup-ADR und #84: Velero muss dort auf Datei-Backup (restic/kopia) ausweichen. |
| Objektspeicher | Velero-Bucket vorhanden (`infra/velero`) | Azure Local bringt **keinen S3-Dienst** mit. Ziel wäre Azure Blob Storage in Azure | Die Randbedingung „außerhalb des Clusters" aus dem Backup-ADR erfüllt sich von selbst. Dafür hängt das Backup am WAN-Link. |

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
   - **Bleiben `SP_SESSION_KEY` und `SP_KIOSK_TOKEN`.** Zwei Werte, die sich
     nie ändern dürfen. *Vorschlag: für die erste Runde von Hand angelegt, als
     dokumentierter Schritt null.* Zwei unveränderliche Werte rechtfertigen
     keinen Tresor, und External Secrets gegen Key Vault ist später
     nachrüstbar, ohne dass sich an den Manifesten etwas ändert — die Base
     referenziert sie ohnehin nur per Namen (#78: „referenced, never
     rendered").

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
7. **Hostname und DNS.** Nicht technisch schwierig, aber mit der längsten
   Halbwertszeit im ganzen Vorhaben: **Der Name muss den Cluster-Neubau
   überleben, und er muss über Übergangs- und Zielumgebung tragen.** ADR-0004
   legt uns auf WebAuthn fest, und Passkeys hängen an der Relying-Party-ID,
   also am Hostnamen. Ein Wechsel der URL entwertet sie — auch bei
   fehlerfreiem Datenbank-Restore. #74 löst sich mit Gateway API sonst in zwei
   Zeilen Profil auf; diese eine Zeile bleibt und gehört nicht uns. Zu klären,
   wer die Zone besitzt.

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
   Controller, Loadbalancer, StorageClass. Die kustomize-Base bleibt für beide
   Umgebungen dieselbe. Das ist Invariante 1, eine Ebene höher gezogen.
3. **Der Objektspeicher-Zugang läuft über Managed Identity**, nicht über ein
   Geheimnis in Git. Damit bleibt als Bootstrap-Problem nur, was die Anwendung
   selbst braucht, und das sind zwei Werte.

Offen und nicht allein entscheidbar: der Hostname (Punkt 7) und die Art, wie
Argo CD auf Azure Local installiert wird (Punkt 6).

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
