"""
The CloudNativePG Cluster — a separate entry point, deliberately.

main.k renders no Cluster, and the reason is worth repeating: the database
outlives every revision of the application, so a base that carries it is a base
whose removal can prune the data. Under ArgoCD that is not hypothetical; it is
what `prune: true` does when an Application stops managing a resource.

This file exists for the path where nothing prunes — a test cluster, a first
bring-up, fault-finding. Keeping it a separate entry point rather than a flag on
main.k is what makes that safe: kcl:publish renders main.k, so no value anyone
sets can put a Cluster into the published artefact. The invariant holds by
construction rather than by a default.

What it buys is not convenience. The owner role, the database name and the
Service the DSN dials are spread today across a helmfile invocation, a Vault
entry and this module, and keeping the three equal is something a human does
between two windows. Rendered from the same fields, they cannot drift.

Requires the CloudNativePG operator on the cluster; this renders the Cluster CR
only. Render with `task kcl:database`, or let `task kcl:up` apply it in the
right order together with everything else.
"""

import labels

# Private throughout. An entry point emits every top-level name that is not
# underscore-prefixed, so a bare `config = labels.config` here would ship the
# whole schema instance alongside the Cluster. main.k avoids this by importing
# only modules; this file has just one resource and keeps it local instead.
_config = labels.config

# storageClass is omitted rather than sent empty: CNPG treats an absent key as
# "the cluster's default", and an empty string as a StorageClass with no name.
_storage = {
    size: _config.dbStorageSize
    if _config.dbStorageClass:
        storageClass: _config.dbStorageClass
}

# initdb.secret points at the Secret this module also builds the DSN from —
# <name>-db, kubernetes.io/basic-auth. CNPG reads username and password from it
# instead of generating a password of its own, which is the only reason the DSN
# and the role can be guaranteed to agree.
_initdb = {
    database: _config.dbName
    owner: labels.dbOwner
    secret: {
        name: labels.dbSecretName
    }
}

# The PVC carries an ownerReference to the Cluster, so deleting the Cluster
# object deletes the data — there is no CNPG-side retention flag that survives
# it. Under an Application with `prune: true` and the resources-finalizer, two
# ordinary GitOps events reach the data that way: the resource leaving the
# render, and the Application itself being deleted. They are separate events
# and one sync-option does not imply the other, hence both.
#
# The comment at the top of this file argues the Cluster stays out of the
# published base so nothing prunes it. This is that argument finished: the
# guard travels on the object, so it also holds for whoever re-adds the Cluster
# to a chart of their own, and for `task kcl:database` applied by hand.
#
# The cost is deliberate and not a bug: the database then has to be removed by
# hand. That is the right trade for an object whose deletion is not recoverable
# from git.
#
# config.annotations is spread after the guard rather than before it. A caller
# adding its own annotations gets them; a caller setting this same key gets a
# render error rather than a silently unprotected Cluster, because KCL's dict
# union rejects a conflicting value instead of letting the later one win.
_annotations = {
    "argocd.argoproj.io/sync-options": "Prune=false,Delete=false"
    **_config.annotations
}

_dbCluster = {
    apiVersion: "postgresql.cnpg.io/v1"
    kind: "Cluster"
    metadata: {
        name: _config.dbClusterName
        namespace: _config.namespace
        labels: labels.databaseLabels
        annotations: _annotations
    }
    spec: {
        instances: _config.dbInstances
        imageName: _config.dbImage
        enableSuperuserAccess: _config.dbSuperuserAccess
        bootstrap: {
            initdb: _initdb
        }
        storage: _storage
    }
}

manifests = [_dbCluster]
