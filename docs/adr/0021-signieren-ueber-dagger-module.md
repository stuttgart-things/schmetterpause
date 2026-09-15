# ADR-0021: Signieren und Prüfen über die geteilten Dagger-Module, das Token holt der Workflow

- **Status:** accepted
- **Datum:** 2026-09-15
- **Betrifft:** Betrieb, Deployment, Lieferkette
- **Bezug:** supersedes `0020-signieren-und-pruefen` in Entscheidung 3
  („Signiert wird im Workflow, nicht in Dagger"); alles andere in ADR-0020 gilt
  weiter. Setzt #254 um. Die Module kommen aus stuttgart-things/dagger#376,
  #377, #378 und #379, veröffentlicht in v0.130.0 und v0.131.0.

## Kontext

ADR-0020 hat das Signieren in `ci.yml` gelegt, als Shell zwischen
heruntergeladenen Werkzeugen: cosign, trivy und oras, drei Versionen, die
Renovate über einen eigenen Manager bewegen musste, und rund 120 Zeilen in
`sign-image`, `sign-kustomize`, `verify-artefacts` und `verify-image-tags`. Die
Gründe waren damals richtig: das OIDC-Token gehört dem Runner, und in
`stuttgart-things/dagger` konnte nichts signieren.

Seit v0.131.0 kann es. Das `cosign`-Modul signiert, attestiert und prüft,
`trivy` schreibt eine SBOM für eine Plattform, `crane` löst Tags zu Digests auf
und vergleicht zwei, `kyverno` testet eine Policy gegen erwartete Ergebnisse.
Vor dieser Entscheidung am 15.09. gegen die Artefakte geprüft, die `main` in
#252 signiert hat:

| Aufruf | Ergebnis |
| --- | --- |
| `cosign verify`, Image und Kustomize-Artefakt | verifiziert, Subject `…/ci.yml@refs/heads/main` |
| `cosign verify-attestation --predicate-type cyclonedx` | die SBOM, 32 Komponenten |
| `cosign verify-refuses`, fremde Identität | abgelehnt, „refused, as it must" |
| `cosign sign` mit Tag statt Digest | abgelehnt, bevor ein Token gebraucht wird |
| `crane digest`, `crane same-digest` | die signierten Digests; ein fehlender Tag ist ein Fehler |
| `trivy sbom` | dieselben 32 Pakete wie die in CI attestierte SBOM |
| `kyverno test` über `policy/tests` | 8 von 8 Erwartungen |

Nicht geprüft: `sign` und `attest` mit einem echten Token. Das kann nur ein
Lauf in dieser Pipeline.

## Entscheidung

1. **Signieren, Attestieren und Prüfen laufen über die Module**, aufgerufen mit
   `dagger call -m` direkt aus den Schritten in `ci.yml`. Die Jobs installieren
   kein Werkzeug mehr; `COSIGN_VERSION`, `TRIVY_VERSION` und `ORAS_VERSION`
   verschwinden aus `ci.yml` und mit ihnen der Renovate-Manager dafür.
2. **Das Token holt der Workflow, nicht das Modul.** Der Schritt fordert bei
   Actions ein Token mit Audience `sigstore` an, maskiert es und reicht es als
   Secret weiter. `ACTIONS_ID_TOKEN_REQUEST_*` gelangen nie in einen Container.
   Ein frisches Token pro signierendem Aufruf.
3. **Die Identität bleibt, wie sie ist:**
   `https://github.com/stuttgart-things/schmetterpause/.github/workflows/ci.yml@refs/…`.
   Policy, Taskfile, Betriebsseite und `scripts/signing_test.sh` ändern sich
   dafür nicht. Vor dem Merge muss ein Lauf auf dem Pull Request dieses Subject
   in `verify-artefacts` zeigen.
4. **Die Modul-Versionen stehen im Taskfile**, wie `KCL_MODULE`:
   `COSIGN_MODULE`, `CRANE_MODULE`, `TRIVY_MODULE` und `KYVERNO_MODULE`, alle
   auf einem Tag. CI liest sie von dort; `task verify:signature`,
   `task verify:sbom` und `task policy:test` rufen dieselben Referenzen auf.
   Renovate bewegt die vier gemeinsam.
5. **Die Policy wird in der Pipeline getestet.** Der Job `policy-test` lässt
   `kyverno test` über `policy/tests` laufen, mit der Kyverno-Version, die auf
   `homerun2-test1` läuft (`KYVERNO_VERSION` im Taskfile), und mit
   `--warnings-as-errors`.
6. **Die Dagger-Engine in `ci.yml` geht auf 0.21.9**, die Version, die die
   Module deklarieren. Eine Engine für alle Aufrufe der Datei, nicht zwei.

## Begründung

### Warum die Einwände aus ADR-0020 nicht mehr tragen

*„Das OIDC-Token gehört dem Runner."* Das stimmt weiter, und deshalb fordert
der Runner es an. Das Modul bekommt ein kurzlebiges Token, nicht die
Berechtigung, eins anzufordern.

*„`task release` sähe aus, als könnte es signieren."* Nur, wenn das Signieren
in `dagger/main.go` läge. Es liegt in keiner Funktion dieses Repositorys: die
Pipeline ruft ein geteiltes Modul auf, und das Taskfile bietet nur die Prüfung
an, die ohne Token funktioniert.

*„Das Kustomize-Artefakt kommt aus einem geteilten Workflow."* Bleibt so, und
`sign-kustomize` bleibt ein Schritt danach. Nur der Digest kommt jetzt aus
`crane` statt aus `oras | sed`.

### Warum `dagger call -m` aus dem Workflow und nicht aus `dagger/main.go`

`dagger/main.go` ist die Pipeline der Anwendung: `task ci` tut lokal dasselbe
wie in CI. Signieren ist dort nicht nachzustellen — ohne Token geht es
nirgends —, und die Module als Abhängigkeit in `dagger.json` einzutragen hieße,
dass jede Änderung an ihnen den Build der Anwendung berührt. Die Aufrufe
gehören zu den Schritten, die nach dem Push über veröffentlichte Artefakte
laufen, wie der Scan.

### Warum kein geteilter Workflow

Fulcio schreibt den `job_workflow_ref` dessen, der das Token anfordert, als
Subject Alternative Name ins Zertifikat (`pkg/identity/github/principal.go` in
sigstore/fulcio). Forderte ein Template in `github-workflow-templates` das Token
an, signierte jedes Repository, das das Template nutzt, als dieselbe Identität.
Das Modul nimmt das Token nur entgegen; angefordert wird es hier.

### Warum der Policy-Test in der Pipeline

Der erste Entwurf der Policy wurde von Kyverno 1.19.1 abgelehnt und übersah
Digest-Referenzen, und beides fiel nur auf, weil jemand von Hand geprüft hat
(ADR-0020). `kyverno test` prüft beide Richtungen: was abgewiesen werden muss
und was durchgehen muss. Die Version kommt aus dem Taskfile und nicht aus dem
Modul, weil der Test eine Aussage über den Cluster ist, auf dem die Policy
läuft.

## Konsequenzen

- **Positiv:** Keine heruntergeladenen Binaries mehr in `ci.yml`. Offener Punkt
  6 aus ADR-0020 — Werkzeuge ohne Prüfsumme — gilt für cosign und trivy nicht
  mehr in dieser Form: sie kommen aus den Release-Images, die die Module pinnen.
  Nach Tag, nicht nach Digest.
- **Positiv:** Die SBOM schreibt dasselbe trivy-Image wie der Scan, heute
  beide 0.64.1.
- **Positiv:** `verify-refuses` ist strenger als der Schritt, den es ersetzt.
  `if cosign verify …; then exit 1` wertete jeden Fehler als Ablehnung, auch
  eine nicht erreichbare Registry. Das Modul verlangt, dass cosign wegen der
  Identität ablehnt.
- **Positiv:** `task verify:signature` und `task verify:sbom` brauchen kein
  lokales cosign und kein jq mehr, nur Dagger.
- **Kosten:** Eine Korrektur an einem Modul ist ein Release in
  `stuttgart-things/dagger` und ein Pin-Bump hier. Der Trivy-404 aus #252 wäre
  so ein Fall gewesen.
- **Kosten:** Jeder dieser Jobs startet eine eigene Dagger-Engine.
- **Einschränkung:** Der Policy-Test hängt an zwei veröffentlichten Images,
  `v0.8.0` (unsigniert) und `3c7f3c7` (der erste von `main` signierte Stand).
  Wird eins gelöscht, wird der Job rot. Er braucht außerdem Netz zu `ghcr.io`
  und Rekor.

## Offene Punkte

1. **`sign` und `attest` mit echtem Token** zeigen sich erst im ersten Lauf auf
   diesem Pull Request, danach auf `main` und auf einem Release-Tag.
2. **Die signierten Fixtures auf ein Release umstellen**, sobald eins signiert
   ist (v0.9.0, #253). Ein Release-Tag bleibt mit Absicht, ein Snapshot nur,
   weil ihn niemand löscht.
3. **Die Übung am Cluster** aus ADR-0020 bleibt offen.
