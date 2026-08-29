"""
ExternalSecrets — the only way a secret value reaches this application.

The rule this file exists to enforce: the schema accepts a *path* into the
cluster's secret store, never a value. There is no field anywhere in schema.k
that could hold SP_SESSION_KEY, and no default that could invent one.

That is not tidiness. A changed session key does not fail — it silently forgets
every player, and TestARestartWithADifferentKeyForgetsEverybody exists to prove
it. A value in a profile file is a value in git, and a rendered Secret with a
generated default would log the whole office out on every render. A wrong Vault
path, by contrast, fails loudly at sync time. Loud beats silent here.

Two Secrets rather than one, and the split is least privilege rather than
tidiness: the migration initContainer mounts only the database Secret and never
sees the cookie secret. config.Load and ValidateForServe are separate
(internal/config/config.go) so that this costs nothing to arrange.

Rendered only when secretsMode is "external". Set it to "existing" where the
Secrets are put in place by something else — both are then expected under the
same names.
"""

import labels

config = labels.config

_store = {
    kind: config.secretStoreKind
    name: config.secretStoreName
}

# The application's own secrets. SP_KIOSK_TOKEN appears only when the kiosk is
# wanted: without the variable the kiosk routes are never registered, so it does
# not exist rather than existing unlocked.
appExternalSecret = {
    apiVersion: config.externalSecretApiVersion
    kind: "ExternalSecret"
    metadata: {
        name: labels.appSecretName
        namespace: config.namespace
        labels: labels.commonLabels
    }
    spec: {
        refreshInterval: config.refreshInterval
        secretStoreRef: _store
        target: {
            name: labels.appSecretName
            creationPolicy: "Owner"
        }
        data: [
            {
                secretKey: "SP_SESSION_KEY"
                remoteRef: {
                    key: config.vaultPath
                    property: config.vaultKeySessionKey
                }
            }
            if config.kioskEnabled:
                {
                    secretKey: "SP_KIOSK_TOKEN"
                    remoteRef: {
                        key: config.vaultPath
                        property: config.vaultKeyKioskToken
                    }
                }
        ]
    }
} if labels.renderExternalSecrets else None

# The database Secret does double duty, which is why it is templated rather
# than passed through.
#
# CloudNativePG takes an existing kubernetes.io/basic-auth Secret as the owner's
# credentials at bootstrap (cluster/appSecretName in the catalog entry), and
# wants username and password. The application wants one SP_DATABASE_URL and
# cannot assemble one from parts. Owning the Secret satisfies both, and removes
# a dependency on whether CNPG's generated <cluster>-app Secret carries a
# ready-made uri key — the catalog documents only username, password and dbname.
#
# Vault holds the password. Everything else in the URL — host, port, database,
# sslmode — comes from this module, where it is a variable and not a secret.
#
# One sharp edge, stated rather than discovered: the password is interpolated
# into a URL without escaping. Keep it alphanumeric in Vault. A password
# containing @ / : or ? produces a DSN that parses as something else.
_dsn = "postgresql://{{ .username }}:{{ .password }}@" + labels.dbHost + ":" + str(config.dbPort) + "/" + config.dbName + "?sslmode=" + config.dbSSLMode

dbExternalSecret = {
    apiVersion: config.externalSecretApiVersion
    kind: "ExternalSecret"
    metadata: {
        name: labels.dbSecretName
        namespace: config.namespace
        labels: labels.commonLabels
    }
    spec: {
        refreshInterval: config.refreshInterval
        secretStoreRef: _store
        target: {
            name: labels.dbSecretName
            creationPolicy: "Owner"
            template: {
                engineVersion: "v2"
                type: "kubernetes.io/basic-auth"
                data: {
                    username: "{{ .username }}"
                    password: "{{ .password }}"
                    SP_DATABASE_URL: _dsn
                }
            }
        }
        data: [
            {
                secretKey: "username"
                remoteRef: {
                    key: config.vaultPath
                    property: config.vaultKeyDBUsername
                }
            }
            {
                secretKey: "password"
                remoteRef: {
                    key: config.vaultPath
                    property: config.vaultKeyDBPassword
                }
            }
        ]
    }
} if labels.renderExternalSecrets else None
