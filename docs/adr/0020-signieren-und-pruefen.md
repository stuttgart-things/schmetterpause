# ADR-0020: Schlüsselloses Signieren, SBOM am Digest — und Kyverno prüft, nicht ein Mensch

- **Status:** accepted — Entscheidung 3 ersetzt durch ADR-0021
- **Datum:** 2026-09-15
- **Betrifft:** Betrieb, Deployment, Lieferkette
- **Bezug:** setzt #230 um (Strang 4 aus #194, Definition of Done Punkt 5).
  Baut auf #86 (Trivy und govulncheck), #177/#179 (Tags, die ihr Artefakt
  nicht benannt haben), #184/#187 (Pinning per SHA) und #80 (das
  Kustomize-Artefakt) auf. Nennt für Azure ADR-0016. Ersetzt kein ADR.

## Kontext

`grep -ri 'cosign\|sbom\|attest\|syft'` über `.github/`, `dagger/` und
`Taskfile.yml` fand am 12.09. **nichts**. Es gab keine Signatur, keine
Stückliste und keine Attestierung in diesem Repository.

Trivy und govulncheck *lesen* das Artefakt, sie bürgen nicht dafür. Ein Scan
sagt „ich habe hier keine bekannten Lücken gefunden", nicht „das hier ist das,
was gebaut wurde".

Die Phase, aus der #230 stammt, beginnt mit
`alertmanager_notifications_failed_total = 13913` — jeder je ausgelöste Alarm
am ersten Sprung abgewiesen, von einem Aufbau, der konfiguriert aussah. Eine
Signatur, die niemand prüft, ist derselbe Gegenstand. Deshalb sind Signieren,
SBOM und Prüfen **eine** Entscheidung und nicht drei: das Prüfen ist der Punkt,
die anderen beiden machen es nur möglich.

Zwei Artefakte, und beide brauchen es:

| | |
| --- | --- |
| `ghcr.io/stuttgart-things/schmetterpause` | das Image, ein Manifest über amd64 und arm64 |
| `ghcr.io/stuttgart-things/schmetterpause-kustomize` | der gerenderte Manifest-Satz, den **Argo** zieht |

Der zweite ist der, den ein Angreifer lieber hätte. Eine Image-Signatur allein
schützt die Hälfte, die niemand angreifen würde.

Am 13.09. auf `homerun2-test1` nachgesehen: **Kyverno läuft dort bereits**, aus
dem argocd-Katalog (`infra/kyverno/install`), mit eigenen `ClusterPolicy`-
Objekten. Die in #230 vermutete Plattform-Anfrage entfällt damit. Am 15.09.
nachgelesen: Version 1.19.1, und `policies.kyverno.io/v1` wird ausgeliefert.

## Entscheidung

1. **Schlüssellos signiert, nicht mit einem Schlüssel aus Vault.** Der Job
   holt sich ein OIDC-Token von Actions, Fulcio tauscht es gegen ein
   Zertifikat mit zehn Minuten Gültigkeit, Rekor hält den Nachweis. Signiert
   wird als
   `https://github.com/stuttgart-things/schmetterpause/.github/workflows/ci.yml@refs/*`
   beim Aussteller `https://token.actions.githubusercontent.com`.
2. **Signiert wird der Digest, nie ein Tag.** Der `release`-Job gibt die
   Referenz mit Digest schon zurück — genau dafür, dass der Scan liest, was
   gepusht wurde. Die Signatur hängt an derselben Stelle.
3. **Signiert wird im Workflow, nicht in Dagger**, für beide Artefakte:
   `sign-image` nach dem Push, `sign-kustomize` nach dem geteilten Workflow.
4. **Die Stückliste hängt als CycloneDX-Attestierung am selben Digest.**
   Erzeugt von Trivy, das den Digest ohnehin zieht. Kein zweites Werkzeug.
   Kein Actions-Artefakt, das mit dem Lauf verfällt.
5. **Geprüft wird an zwei Stellen, und beide sind benannt:**
   - **Am Cluster** durch eine Kyverno-`ImageValidatingPolicy`
     (`policies.kyverno.io/v1`, `policy/verify-image-signature.yaml`),
     begrenzt auf `schmetterpause` und `schmetterpause-pr-*`, für Referenzen
     per Tag und per Digest. Das ist die stärkere Aussage: sie betrifft das
     Image, das der Kubelet gleich startet.
   - **Im Lieferweg** durch den Job `verify-artefacts`, der beide Artefakte
     nach dem Signieren prüft. Für das Kustomize-Artefakt ist er die einzige
     Prüfung, weil Admission es nie zu sehen bekommt.
6. **Die Policy startet auf `Audit` und geht nach drei sauberen Releases auf
   `Deny`** — die Haltung, die Trivy hier schon eingenommen hat. Mit dem
   Wechsel kommen `mutateDigest` und `verifyDigest` dazu. Der Tag, an dem sie
   aufhört, eine Messung zu sein, wird in `docs/supply-chain.md` festgehalten.
7. **Der CI-Job prüft dagegen ab dem ersten Lauf blockierend**, und das ist
   kein Widerspruch zu Punkt 6. Trivys Urteil hängt davon ab, was die Welt
   über Nacht gefunden hat; dieser Job prüft eine Signatur, die derselbe Lauf
   neunzig Sekunden vorher gemacht hat. Wird er rot, ist das Signieren kaputt,
   nicht die Welt.
8. **Azure Container Apps ist von der Prüfung am Laufzeitort ausgenommen** und
   das steht als Satz da, nicht als Lücke: dort gibt es keine Admission (#206),
   die Umgebung läuft auf Zeit (ADR-0016), und was dort startet, ist in CI
   geprüft und sonst nirgends.
9. **Beweis, dass die Prüfung ablehnen kann.** Der letzte Schritt von
   `verify-artefacts` lässt dieselbe Prüfung gegen eine Identität laufen, die
   nicht passen darf, und macht den Build rot, wenn sie durchgeht. Die Übung
   am Cluster — ein unsigniertes Image, das abgewiesen wird — steht als
   Anleitung in `docs/supply-chain.md` und ist noch nicht durchgeführt.
10. **Die Signatur-Identität steht an vier Stellen, und ein Test vergleicht
    sie.** Pipeline, Taskfile, Policy und Betriebsseite nennen denselben
    Aussteller und dieselbe verankerte Regexp als Unterzeichner.
    `scripts/signing_test.sh` vergleicht sie, prüft, dass die Policy genau
    einen Unterzeichner nennt, und läuft in der Lint-Stufe. Eine der vier zu
    verbreitern — oder der Policy einen zweiten Unterzeichner danebenzustellen
    — ist der leise Weg, auf dem dieses Tor aufhört, eines zu sein.

## Begründung

### Warum schlüssellos

Ein Schlüssel in Vault nutzt Maschinerie, die diese Organisation schon
betreibt (#84) — und kostet ein Geheimnis, das rotiert werden muss und das
nicht rotiert wird, indem man es vergisst. Schlüssellos gibt es nichts zu
rotieren und nichts zu verlieren; der Preis ist ein öffentlicher Eintrag im
Transparenzlog pro Signatur. Für ein öffentliches Paket ist das kein Preis,
und es ist besser, das entschieden zu haben, als dass es passiert.

Der Job braucht dafür `id-token: write`. Er hatte bisher `contents: read` und
`packages: write`; die dritte Berechtigung ist der ganze Unterschied.

### Warum im Workflow und nicht in Dagger

`ci.yml` sagt über sich, es sei *bewusst dünn* und enthalte *keine
Build-Logik*. Die Regel bleibt heil: Signieren ist keine Build-Logik. Es
passiert **nach** dem Push, über ein Artefakt, das es schon gibt — dieselbe
Klasse wie der Scan und wie `verify-image-tags`, die beide längst hier stehen.

Dazu zwei praktische Gründe. Das OIDC-Token gehört dem Runner; es an Dagger zu
reichen hieße, `task release` so aussehen zu lassen, als könnte es signieren,
was es nicht kann. Und das Kustomize-Artefakt wird von einem geteilten
Workflow gepusht, den dieses Repository nicht besitzt — dort geht nur ein
Schritt *danach*. Die beiden Artefakte werden also nicht an derselben Stelle
signiert. #230 lässt das zu, „solange es bemerkt und nicht entdeckt wird";
der Kommentar über `sign-kustomize` ist das Bemerken.

### Warum Admission *und* ein CI-Job

Sie prüfen nicht dasselbe. Der CI-Job prüft, was wir veröffentlicht haben;
Admission prüft, was der Cluster gleich startet. Das eine ersetzt das andere
nicht — und für das Kustomize-Artefakt gibt es nur das erste, weil Argo es
zieht und der Kubelet nie.

### Warum eine `ImageValidatingPolicy` und keine `ClusterPolicy`

Der erste Entwurf war eine `ClusterPolicy` mit `verifyImages`. Kyverno 1.19.1
auf `homerun2-test1` hat sie am 15.09. im Server-Dry-Run abgelehnt —
`mutateDigest` ist unter `Audit` dort nicht erlaubt — und beantwortet jede
`kyverno.io/v1`-`ClusterPolicy` mit dem Hinweis, die Art sei veraltet. Die
`ImageValidatingPolicy` ist der dort genannte Nachfolger, wird auf dem Cluster
in `v1` ausgeliefert und nimmt den Unterzeichner als Regexp. Damit steht an
allen vier Stellen dieselbe Zeichenkette. Vorher stand in der Policy ein Glob,
und dass Glob und Regexp dasselbe meinen, sieht beim Lesen niemand.

Mit dem Kyverno-CLI 1.19.1 gegen je einen Pod geprüft: per Tag, per Digest,
per Tag und Digest, mit dem Image nur im Init-Container und im
Preview-Namespace — alle gemeldet. Ein Pod nur mit Postgres und einer außerhalb
der beiden Namespaces — nicht angefasst. Dabei fiel auch auf, dass der Glob
`schmetterpause:*` des ersten Entwurfs eine reine Digest-Referenz gar nicht
getroffen hat: ein Pod mit `schmetterpause@sha256:…` wäre ungeprüft
durchgegangen. Deshalb stehen jetzt beide Schreibweisen in
`matchImageReferences`.

### Warum eine Policy für Büro und Previews zusammen

Ein Preview-Image baut derselbe Workflow und signiert dieselbe Identität. Ein
Preview, das an dieser Prüfung scheitert, würde auch in der echten Umgebung
scheitern — dann lieber dort, wo es billig ist.

### Warum `mutateDigest` erst mit `Deny`

Ohne das Umschreiben prüft die Policy die Signatur an dem, worauf das Tag im
Moment der Admission zeigt, und der Kubelet darf danach etwas anderes ziehen.
Mit dem Umschreiben steht im Pod der Digest, der geprüft wurde, und
`verifyDigest` verlangt ihn dann. Die beiden Einstellungen gehören zusammen.

Unter `Audit` bleiben beide aus. Der Server nimmt die Kombination zwar an, aber
das CLI meldet dann jedes Tag nur als „hat keinen Digest", statt zu sagen, ob
es signiert ist — und was die Admission daraus macht, hat niemand beobachtet.
Eine Messphase, deren Meldungen etwas anderes messen als die Signatur, ist
keine. Eingeschaltet wird es mit dem Wechsel auf `Deny`; die Übung in
`docs/supply-chain.md` ist die Stelle, an der sich zeigt, ob es tut, was es
soll.

## Konsequenzen

- **Positiv:** Wer das Image in der Hand hat, kann ohne Zugang zu diesem
  Repository fragen, woher es kommt und was darin ist. Beides hängt am Digest
  und überlebt jede Aufbewahrungsfrist von Actions.
- **Positiv, ab `Deny`:** `mutateDigest` schließt das Fenster zwischen „Tag
  geprüft" und „Image gezogen" — dieselbe Klasse von Fehler wie #177 und #179,
  nur zur Laufzeit. Bis zum Wechsel steht dieses Fenster offen.
- **Einschränkung: die SBOM beschreibt `linux/amd64`.** Eine Stückliste gilt
  pro Image, nicht pro Index. arm64 kommt aus derselben Quelle und derselben
  Basis, aber das ist eine Behauptung, die niemand geprüft hat.
- **Einschränkung: `failurePolicy: Ignore`.** Erreicht Kyverno Registry oder
  Rekor nicht, wird der Pod zugelassen statt abgewiesen. Für einen Cluster,
  auf dem das Büro spielt, ist das die richtige Voreinstellung — für eine
  Aussage über Abdeckung die falsche. Deshalb benannt.
- **Einschränkung: nur unser Image.** Die Policy greift für
  `ghcr.io/stuttgart-things/schmetterpause:*` und `…@*`. Postgres daneben im
  selben Namespace ist nicht geprüft und wird auch nicht als geprüft
  ausgegeben. Sie verhindert auch nicht, dass ein Pod ein ganz anderes Image
  startet: sie sagt, dass ein Image, das sich schmetterpause nennt, von uns
  signiert ist — nicht, dass dort nur schmetterpause läuft.
- **Einschränkung: Signaturen von Pull Requests werden nicht aufgeräumt.**
  `cleanup-pr-artifacts.yml` löscht Versionen mit `pr-<n>-*`; cosign legt die
  Signatur unter `sha256-<digest>.sig` ab. Pro Build bleiben zwei kleine
  Objekte liegen — die Deponie aus #20 im Kleinen.
- **Positiv:** Die Lint-Stufe merkt, wenn die vier Kopien der
  Signatur-Identität auseinanderlaufen oder die Policy einen zweiten
  Unterzeichner bekommt. Ohne das wäre eine verbreiterte Regexp in der Policy
  eine grüne Pipeline und eine Prüfung, die nichts mehr aussagt — dieselbe
  Klasse wie #177 und #179, nur an der Stelle, die das verhindern soll.
- **Kosten:** zwei Jobs mehr pro Lauf, ein Trivy-Pull mehr, und zwei weitere
  gepinnte Werkzeugversionen in `ci.yml`, die Renovate bewegen muss — über
  einen eigenen Manager in `renovate.json`, weil die Kommentare in `ci.yml`
  sonst niemand liest. Der Major-Sprung von cosign braucht eine Freigabe, weil
  ein grüner `verify-artefacts` nicht zeigt, dass Kyverno liest, was cosign 3
  schreibt.
- **Nötig von Hand:** die Policy anwenden. Sie liegt nicht im Kustomize-Base,
  weil sie clusterweit ist und jeder Preview-Namespace sie sonst mit
  installieren und wegräumen würde (`policy/README.md`) — dieselbe Trennung,
  die ADR-0019 für die Backup-Konfiguration zieht.

## Offene Punkte

1. **Die Übung am Cluster ist noch nicht gelaufen.** Solange sie nicht
   gelaufen ist, gilt der Satz aus #230 für die Admission-Hälfte weiter: eine
   Prüfung, die nie etwas abgelehnt hat, ist von einer nicht zu unterscheiden,
   die es nicht kann. Der CI-Job hat seine Hälfte davon ab dem ersten Lauf.
   Mit der Übung unter `Deny` zeigt sich auch, ob `mutateDigest` an der
   Admission tut, was es soll.
2. **Wer die Policy reconciled.** Heute von Hand. Sie in den argocd-Katalog zu
   legen, wäre der nächste Schritt — und die Frage, wem sie dann gehört.
3. **Der Lieferweg von Argo.** Nichts prüft die Signatur des
   Kustomize-Artefakts in dem Moment, in dem Argo es zieht. Dafür bräuchte es
   eine Prüfung vor Argos OCI-Pull, die in dieser Organisation niemand
   betreibt.
4. **Eine zweite Attestierung für arm64.**
5. **Aufräumen der Signaturen geschlossener Pull Requests** — braucht eine
   Änderung im geteilten Workflow.
6. **Die Werkzeuge selbst.** cosign und trivy werden per Versionsstring
   geladen und nicht gegen eine Prüfsumme gehalten — wie oras daneben.
