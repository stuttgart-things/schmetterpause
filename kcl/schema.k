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
    
    # A placeholder for local renders. CI overrides it with the reference the
    # release job published, digest and all — see the README.
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
    
    # A CloudNativePG Cluster is NOT rendered here. It outlives every app
    # revision, and a base that carries it is a base whose removal can prune
    # the database. It is its own Argo Application from the catalog entry
    # infra/cloudnative-pg/cluster, deployed into this same namespace.
    #
    # These values describe how to reach it, not how to create it.
    dbClusterName: str = "schmetterpause-db"
    dbName: str = "schmetterpause"
    dbOwner: str = "schmetterpause"
    dbPort: int = 5432
    dbSSLMode: str = "require"

    # Empty means "<dbClusterName>-rw.<namespace>.svc", the primary Service the
    # CNPG Cluster publishes.
    dbHost: str = ""

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
        logLevel in ["debug", "info", "warn", "error"], "logLevel must be one of debug, info, warn, error"
        secretsMode in ["external", "existing"], "secretsMode must be 'external' or 'existing'"
        secretsMode != "external" or secretStoreName != "", "secretsMode 'external' needs secretStoreName"
        secretsMode != "external" or vaultPath != "", "secretsMode 'external' needs vaultPath"
        secretStoreKind in ["ClusterSecretStore", "SecretStore"], "secretStoreKind must be ClusterSecretStore or SecretStore"
        externalSecretApiVersion in ["external-secrets.io/v1", "external-secrets.io/v1beta1"], "externalSecretApiVersion must be external-secrets.io/v1 or external-secrets.io/v1beta1"
        not httpRouteEnabled or gatewayName != "", "httpRouteEnabled needs gatewayName"
        not httpRouteEnabled or host != "" or clusterDomain != "", "httpRouteEnabled needs either host or clusterDomain"

