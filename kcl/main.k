"""
Entry point: every resource, with the ones that are switched off filtered out.

The resources come out as a list under one key rather than as a YAML stream,
and that is a contract rather than a style choice.

github.com/stuttgart-things/dagger/kcl — the module that turns this into a
kustomize base and pushes it as an OCI artifact — post-processes the render
with a fixed sed chain that assumes exactly this shape: drop the first line,
de-indent by two, turn "- " at column zero into a document separator. Handed a
yaml_stream it deletes the first resource's apiVersion and lifts every
metadata field to the top level. The result still parses as YAML, so nothing
errors; it just applies something else. Verified by running the chain.

Use `task kcl:render` rather than `kcl run` directly: it applies the same
normalisation, so what is rendered locally is what the OCI artifact carries.

No Namespace is emitted. Several Applications can share a workload namespace,
and when more than one ships a Namespace resource ArgoCD flags it as a
SharedResource — then one Application's prune cycle deletes the namespace out
from under the others. The consuming Application sets CreateNamespace=true
instead. Learned from stuttgart-things/homerun2-omni-pitcher#132.

No CloudNativePG Cluster either, for a related reason with worse consequences:
the database outlives every revision of the application, and a base that
carries it is a base whose removal can prune the data. It is its own Argo
Application from infra/cloudnative-pg/cluster, into this same namespace, at a
lower sync-wave so it exists before the migration initContainer runs.
"""

import serviceaccount
import configmap
import externalsecret
import deploy
import service
import httproute

_resources = [
    serviceaccount.serviceAccount
    configmap.configMap
    externalsecret.appExternalSecret
    externalsecret.dbExternalSecret
    deploy.deployment
    service.service
    httproute.httproute
    httproute.httprouteRedirect
]

manifests = [r for r in _resources if r]
