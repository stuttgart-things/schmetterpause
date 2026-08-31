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

_dbCluster = {
    apiVersion: "postgresql.cnpg.io/v1"
    kind: "Cluster"
    metadata: {
        name: _config.dbClusterName
        namespace: _config.namespace
        labels: labels.databaseLabels
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
