"""
Typed configuration for the schmetterpause manifests.

Everything is a variable. No hostname, namespace, gateway, cluster domain or
image reference is baked in, so the same module renders for a laptop cluster,
the office cluster and a preview namespace.

Nothing here can hold a secret value, and nothing ever should. See
externalsecret.k for why that is a rule rather than a preference.
"""

schema Schmetterpause:
    """Configuration for the schmetterpause Kubernetes manifests."""
    # ── Identity ────────────────────────────────────────────────────────────
    name: str = "schmetterpause"
    namespace: str = "schmetterpause"

    # ── Image ───────────────────────────────────────────────────────────────
    
    # A placeholder for local renders, and only that: ":latest" moves in this
    # repository's CI only on a v* tag, and there has been no release, so the
    # reference below does not resolve in the registry. task kcl:publish always
    # overrides it with the tag the artefact itself carries, and refuses when
    # that image is not there — so a placeholder cannot reach a published
    # artefact.
    image: str = "ghcr.io/stuttgart-things/schmetterpause:latest"
    imagePullPolicy: str = "IfNotPresent"

    # ── Scale ───────────────────────────────────────────────────────────────
    
    # One replica, and the check below holds it there.
    # internal/repository/postgres/migrate.go calls goose.UpContext through the
    # package-level API, which takes no session lock — that exists only on
    # goose's Provider API. Two pods migrating at once is unsafe in fact, not
    # in theory. Raising this needs an advisory lock or a migration Job first.
    replicas: int = 1

    cpuRequest: str = "50m"
    cpuLimit: str = "500m"
    memoryRequest: str = "64Mi"
    memoryLimit: str = "256Mi"

    # ── Ports ───────────────────────────────────────────────────────────────
    servicePort: int = 80
    containerPort: int = 8080

    # ── Exposure ────────────────────────────────────────────────────────────
    httpRouteEnabled: bool = False
    gatewayName: str = ""
    gatewayNamespace: str = ""

    # The listener to attach to, by name. Not cosmetic: the shared gateway also
    # serves plain HTTP on port 80, and a route without sectionName attaches to
    # both listeners. Over plain HTTP a browser never returns the Secure
    # session cookie, so every request arrives as a stranger and the player is
    # forgotten between clicks — issue #70, rebuilt by a listener.
    gatewaySectionNameHTTPS: str = "https"

    # A second route on the plain listener that redirects to https, so port 80
    # answers with a redirect rather than with the application. Turn it off
    # where the gateway already redirects, or where it has no HTTP listener.
    # Inert without httpRouteEnabled rather than an error, so a profile that
    # does not expose the app at all needs to say nothing about redirects.
    httpRedirectEnabled: bool = True
    gatewaySectionNameHTTP: str = "http"

    # The hostname is assembled from name and clusterDomain rather than pasted
    # in, so one profile value moves the app between clusters. Set host to
    # override the assembly outright.
    clusterDomain: str = ""
    host: str = ""

    # SP_PUBLIC_BASE_URL. Derived from the hostname when empty. It has to be
    # set behind a gateway: left empty the QR sheet reads the address off the
    # incoming request, which is right when the app is reached directly and
    # wrong behind a proxy that terminates TLS — the printed sheet would send
    # every phone to a cluster-internal address.
    publicBaseURL: str = ""

    # ── Application settings ────────────────────────────────────────────────
    logLevel: str = "info"

    # SP_COOKIE_SECURE is deliberately absent: it defaults to true in the code
    # so that a forgotten setting fails closed. compose.yaml is what sets it to
    # false, for local HTTP. Do not add it here.
    
    # The kiosk exists only when SP_KIOSK_TOKEN is set — the routes are not
    # registered otherwise, so it does not exist rather than existing unlocked.
    # Off by default: a kiosk on a server is issue #90, and the measured days
    # are meant to run without one.
    kioskEnabled: bool = False

    # A display name that becomes the first admin at startup. Empty is fine and
    # normal; the person has to have joined already, so this is a second-pass
    # value rather than a first-boot one.
    bootstrapAdmin: str = ""

    # ── Migrations ──────────────────────────────────────────────────────────
    
    # SP_AUTO_MIGRATE stays false and "migrate up" runs as an initContainer.
    # config.Load and ValidateForServe are separate (internal/config/config.go)
    # precisely so this step needs no cookie secret — it gets the database
    # Secret and nothing else.
    migrateInitContainer: bool = True

    # ── Database ────────────────────────────────────────────────────────────
    
    # A CloudNativePG Cluster is NOT part of main.k. It outlives every app
    # revision, and a base that carries it is a base whose removal can prune
    # the database. Under ArgoCD it is its own Application from the catalog
    # entry infra/cloudnative-pg/cluster, into this same namespace.
    #
    # database.k renders one for the path without Argo, where nothing prunes.
    # It is a separate entry point rather than a flag on purpose: kcl:publish
    # renders main.k, so the Cluster cannot reach the published base by anyone
    # setting a value. The invariant holds by construction, not by default.
    #
    # Both read the fields below, which is the point. Today the owner role, the
    # database name and the Service the DSN dials are spread across a helmfile
    # invocation, a Vault entry and this module, and keeping them equal is
    # something a human does. Rendered from one place they cannot drift.
    dbClusterName: str = "schmetterpause-db"
    dbName: str = "schmetterpause"
    dbPort: int = 5432
    dbSSLMode: str = "require"

    # Empty means "<dbClusterName>-rw.<namespace>.svc", the primary Service the
    # CNPG Cluster publishes.
    dbHost: str = ""

    # The owner role. Empty means `name`. Whatever this ends up being has to
    # equal the `username` in the secret store, because that Secret is what
    # CloudNativePG reads the credentials from at initdb — a mismatch creates a
    # role the DSN never uses.
    dbOwner: str = ""

    # Only read by database.k. One instance is lab-shaped: an instance failure
    # is downtime and possible data loss, there being no replica to fail over
    # to. Replicas and backups protect against different things, and the
    # realistic danger on a test cluster is a deliberate rebuild, against which
    # a replica does nothing — so the trigger for raising this is backups, not
    # replicas.
    dbInstances: int = 1
    dbStorageSize: str = "8Gi"

    # Empty means the cluster's default StorageClass.
    dbStorageClass: str = ""

    # Named outright rather than derived from a version number: the CNPG Helm
    # chart maps `version.postgresql` onto an image, and reproducing that
    # mapping here would be a second place for it to be wrong.
    dbImage: str = "ghcr.io/cloudnative-pg/postgresql:17"

    # Off by default: with it disabled the operator blanks the postgres
    # password, so the owner role is the only way in.
    dbSuperuserAccess: bool = False

    # ── Secrets ─────────────────────────────────────────────────────────────
    
    # "external" renders an ExternalSecret per Secret against the cluster's
    # store. "existing" renders neither and expects both Secrets to be present
    # already, by name.
    secretsMode: str = "external"
    secretStoreKind: str = "ClusterSecretStore"
    secretStoreName: str = ""
    refreshInterval: str = "1h"

    # The ExternalSecret API version. A variable because a cluster on an older
    # External Secrets Operator serves v1beta1 and nothing else; v1 is what
    # stuttgart-things/argocd renders today.
    externalSecretApiVersion: str = "external-secrets.io/v1"

    # Where in the store the values live. A path, never a value.
    vaultPath: str = ""

    # Property names inside that path.
    vaultKeySessionKey: str = "session-key"
    vaultKeyKioskToken: str = "kiosk-token"
    vaultKeyDBUsername: str = "username"
    vaultKeyDBPassword: str = "password"

    # ── Escape hatches ──────────────────────────────────────────────────────
    extraEnvVars: {str:str} = {}
    labels: {str:str} = {}
    annotations: {str:str} = {}

    check:
        name != "", "name must not be empty"
        namespace != "", "namespace must not be empty"
        image != "", "image must not be empty"
        replicas == 1, "replicas must be 1: goose takes no session lock, so a second pod migrating concurrently is unsafe"
        servicePort >= 1 and servicePort <= 65535, "servicePort must be a valid port"
        containerPort >= 1 and containerPort <= 65535, "containerPort must be a valid port"
        dbPort >= 1 and dbPort <= 65535, "dbPort must be a valid port"
        dbInstances >= 1, "dbInstances must be at least 1"
        logLevel in ["debug", "info", "warn", "error"], "logLevel must be one of debug, info, warn, error"
        secretsMode in ["external", "existing"], "secretsMode must be 'external' or 'existing'"
        secretsMode != "external" or secretStoreName != "", "secretsMode 'external' needs secretStoreName"
        secretsMode != "external" or vaultPath != "", "secretsMode 'external' needs vaultPath"
        secretStoreKind in ["ClusterSecretStore", "SecretStore"], "secretStoreKind must be ClusterSecretStore or SecretStore"
        externalSecretApiVersion in ["external-secrets.io/v1", "external-secrets.io/v1beta1"], "externalSecretApiVersion must be external-secrets.io/v1 or external-secrets.io/v1beta1"
        not httpRouteEnabled or gatewayName != "", "httpRouteEnabled needs gatewayName"
        not httpRouteEnabled or gatewayNamespace != "", \
            "httpRouteEnabled needs gatewayNamespace: empty attaches the route to a Gateway in the route's own namespace, and a parentRef naming one that is not there gets no status at all rather than a rejection"
        not httpRouteEnabled or host != "" or clusterDomain != "", "httpRouteEnabled needs either host or clusterDomain"

