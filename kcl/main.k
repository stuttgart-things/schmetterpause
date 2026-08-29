"""
Entry point: every resource, with the ones that are switched off filtered out.

Emitted as a YAML stream rather than as a list under one key, so the render is
something `kubectl apply -f` and kustomize can read without post-processing.

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

import manifests as k8s

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

k8s.yaml_stream([r for r in _resources if r])
