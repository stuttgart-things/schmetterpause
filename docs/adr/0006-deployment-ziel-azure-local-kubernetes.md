# ADR-0006: Deployment-Ziel Azure Local / Kubernetes (Entwurf)

- **Status:** Entwurf — noch nicht mit dem Team abgestimmt, kein `accepted`
- **Datum:** 2026-08-21
- **Betrifft:** Deployment, Infrastruktur
- **Phase:** phase-2. Nichts hiervon wird umgesetzt, solange die MVP-Messung
  aus #7 läuft — siehe den entsprechenden Abschnitt in ADR-0007.
- **Verwandt:** ADR-0007 (Backup), ADR-0002 (kein Redis)

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
Lebenszyklus und steht in ADR-0007.

## Offene Fragen (noch nicht entschieden)

1. **GitOps-Mechanismus:** Azure Arc bietet zwei offizielle Wege —
   [Flux v2](https://learn.microsoft.com/en-us/azure/azure-arc/kubernetes/conceptual-gitops-flux2)
   oder eine
   [Argo-CD-Extension](https://learn.microsoft.com/en-us/azure/azure-arc/kubernetes/conceptual-gitops-argocd).
   Tendenz: Argo CD, weil [Kargo](https://kargo.io/) (Continuous-Promotion
   über mehrere Stages) direkt darauf aufsetzt und von denselben Maintainern
   kommt — vermeidet einen zweiten GitOps-Stack für später.
2. **Wie viele Stages?** Mindestens dev/prod plausibel. Kargo lohnt sich erst
   ab mindestens zwei Stufen mit echter Promotion dazwischen — bei nur einer
   Stage reicht Argo CD allein.
3. ~~**Registry**~~ — **entschieden** (Issue #20, 2026-08-21): `ttl.sh` für
   Wegwerf-Builds, `ghcr.io/stuttgart-things/schmetterpause` für alles
   Bleibende. Multi-Arch (amd64+arm64), Push bei jedem `main`-Merge als
   Snapshot, `latest` bewegt sich nur bei echten Tags (`dagger/main.go`,
   `Release`-Funktion). Für dieses ADR heißt das: Die Arc/Argo-CD-Extension
   zieht Images von GHCR — bei privatem Package braucht der Azure-Local-
   Cluster ein `imagePullSecret`, bei öffentlichem nicht. Muss noch geprüft
   werden, wie das GHCR-Package aktuell sichtbar ist.
4. **Skalierung / Redis-Trigger:** ADR-0002 nennt "mehr als eine Replica +
   SSE" als Auslöser für Redis. Sobald das Deployment-Ziel feststeht, prüfen,
   ob Azure Local mit mehr als einer Replica geplant ist — das zieht ADR-0002
   nach vorne.
5. **Secret-Verwaltung:** Wie kommen Zugangsdaten in einen frisch gebauten
   Cluster, bevor Argo CD läuft? Betrifft `imagePullSecret` (Punkt 3) und den
   Objektspeicher-Zugang aus ADR-0007 gleichermaßen — eine Frage, zwei
   Anwendungsfälle. Kandidaten: SOPS/age in Git, External Secrets gegen Azure
   Key Vault, manueller Bootstrap-Schritt.

## Hinweis zur Form

Das Team trackt "Offene Punkte" aus `docs/mvp-plan.md` inzwischen als
GitHub-Issues statt als ADR-Prosa (#17–#20) — ADRs sind für Entscheidungen mit
Bestand, Issues für Fragen, die noch offen sind. Die vier Punkte oben in
diesem ADR sind Kandidaten für denselben Schnitt, sobald das Deployment-Ziel
näher rückt: eigene Issues statt Unterpunkte hier drin.

## Entscheidung

Noch offen. Vorschlag zur Diskussion: Argo-CD-Extension über Azure Arc als
Reconciliation-Schicht, Kargo optional nachziehen sobald eine zweite Stage
existiert. Manifeste/Helm-Chart erst anlegen, wenn AP1–AP3 stabil sind und ein
erstes Image über eine Registry verteilt werden muss.

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
