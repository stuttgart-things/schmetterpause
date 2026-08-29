"""
The one configuration instance, the command-line overrides that shape it, and
the labels every resource carries.

Every field of the schema is overridable, which is what "individually
renderable" means here: one module, one profile file per environment, no
per-environment copy of the manifests.

The enumeration below is unavoidable rather than accidental — KCL resolves
option("config.x") per key, so a spread of option("config") picks up nothing.
The string coercions exist because -D passes every value as a string while a
settings file passes real types; both have to work.
"""

import schema

_name = option("config.name")
_namespace = option("config.namespace")
_image = option("config.image")
_imagePullPolicy = option("config.imagePullPolicy")
_replicas = option("config.replicas")
_cpuRequest = option("config.cpuRequest")
_cpuLimit = option("config.cpuLimit")
_memoryRequest = option("config.memoryRequest")
_memoryLimit = option("config.memoryLimit")
_servicePort = option("config.servicePort")
_containerPort = option("config.containerPort")
_httpRouteEnabled = option("config.httpRouteEnabled")
_gatewayName = option("config.gatewayName")
_gatewayNamespace = option("config.gatewayNamespace")
_gatewaySectionNameHTTPS = option("config.gatewaySectionNameHTTPS")
_httpRedirectEnabled = option("config.httpRedirectEnabled")
_gatewaySectionNameHTTP = option("config.gatewaySectionNameHTTP")
_clusterDomain = option("config.clusterDomain")
_host = option("config.host")
_publicBaseURL = option("config.publicBaseURL")
_logLevel = option("config.logLevel")
_kioskEnabled = option("config.kioskEnabled")
_bootstrapAdmin = option("config.bootstrapAdmin")
_migrateInitContainer = option("config.migrateInitContainer")
_dbClusterName = option("config.dbClusterName")
_dbName = option("config.dbName")
_dbOwner = option("config.dbOwner")
_dbPort = option("config.dbPort")
_dbSSLMode = option("config.dbSSLMode")
_dbHost = option("config.dbHost")
_secretsMode = option("config.secretsMode")
_secretStoreKind = option("config.secretStoreKind")
_secretStoreName = option("config.secretStoreName")
_externalSecretApiVersion = option("config.externalSecretApiVersion")
_refreshInterval = option("config.refreshInterval")
_vaultPath = option("config.vaultPath")
_vaultKeySessionKey = option("config.vaultKeySessionKey")
_vaultKeyKioskToken = option("config.vaultKeyKioskToken")
_vaultKeyDBUsername = option("config.vaultKeyDBUsername")
_vaultKeyDBPassword = option("config.vaultKeyDBPassword")
_extraEnvVars = option("config.extraEnvVars") or {}
_labels = option("config.labels") or {}
_annotations = option("config.annotations") or {}

# A boolean that arrives as the string "false" is still truthy, so every
# boolean is compared rather than assigned. Getting this wrong turns an
# explicit "off" into "on", which is the wrong direction for kioskEnabled.
_bool = lambda v: bool -> bool {
    v in [True, "True", "true", "yes", "1"]
}

config: schema.Schmetterpause = schema.Schmetterpause {
    if _name:
        name = _name
    if _namespace:
        namespace = _namespace
    if _image:
        image = _image
    if _imagePullPolicy:
        imagePullPolicy = _imagePullPolicy
    if _replicas:
        replicas = int(_replicas)
    if _cpuRequest:
        cpuRequest = _cpuRequest
    if _cpuLimit:
        cpuLimit = _cpuLimit
    if _memoryRequest:
        memoryRequest = _memoryRequest
    if _memoryLimit:
        memoryLimit = _memoryLimit
    if _servicePort:
        servicePort = int(_servicePort)
    if _containerPort:
        containerPort = int(_containerPort)
    if _httpRouteEnabled != Undefined and _httpRouteEnabled != None:
        httpRouteEnabled = _bool(_httpRouteEnabled)
    if _gatewayName:
        gatewayName = _gatewayName
    if _gatewayNamespace:
        gatewayNamespace = _gatewayNamespace
    if _gatewaySectionNameHTTPS:
        gatewaySectionNameHTTPS = _gatewaySectionNameHTTPS
    if _httpRedirectEnabled != Undefined and _httpRedirectEnabled != None:
        httpRedirectEnabled = _bool(_httpRedirectEnabled)
    if _gatewaySectionNameHTTP:
        gatewaySectionNameHTTP = _gatewaySectionNameHTTP
    if _clusterDomain:
        clusterDomain = _clusterDomain
    if _host:
        host = _host
    if _publicBaseURL:
        publicBaseURL = _publicBaseURL
    if _logLevel:
        logLevel = _logLevel
    if _kioskEnabled != Undefined and _kioskEnabled != None:
        kioskEnabled = _bool(_kioskEnabled)
    if _bootstrapAdmin:
        bootstrapAdmin = _bootstrapAdmin
    if _migrateInitContainer != Undefined and _migrateInitContainer != None:
        migrateInitContainer = _bool(_migrateInitContainer)
    if _dbClusterName:
        dbClusterName = _dbClusterName
    if _dbName:
        dbName = _dbName
    if _dbOwner:
        dbOwner = _dbOwner
    if _dbPort:
        dbPort = int(_dbPort)
    if _dbSSLMode:
        dbSSLMode = _dbSSLMode
    if _dbHost:
        dbHost = _dbHost
    if _secretsMode:
        secretsMode = _secretsMode
    if _secretStoreKind:
        secretStoreKind = _secretStoreKind
    if _secretStoreName:
        secretStoreName = _secretStoreName
    if _externalSecretApiVersion:
        externalSecretApiVersion = _externalSecretApiVersion
    if _refreshInterval:
        refreshInterval = _refreshInterval
    if _vaultPath:
        vaultPath = _vaultPath
    if _vaultKeySessionKey:
        vaultKeySessionKey = _vaultKeySessionKey
    if _vaultKeyKioskToken:
        vaultKeyKioskToken = _vaultKeyKioskToken
    if _vaultKeyDBUsername:
        vaultKeyDBUsername = _vaultKeyDBUsername
    if _vaultKeyDBPassword:
        vaultKeyDBPassword = _vaultKeyDBPassword
    if _extraEnvVars:
        extraEnvVars = _extraEnvVars
    if _labels:
        labels = _labels
    if _annotations:
        annotations = _annotations
}

# ── Derived values ──────────────────────────────────────────────────────────

# "<name>.<clusterDomain>" unless host says otherwise. Assembling it is what
# lets one profile value move the app between clusters.
hostname = config.host if config.host else ("{}.{}".format(config.name, config.clusterDomain) if config.clusterDomain else "")

# https, always: the route binds to the TLS listener, and SP_COOKIE_SECURE
# defaults to true. A base URL on http would print a QR code leading to a page
# that cannot keep a session.
publicBaseURL = config.publicBaseURL if config.publicBaseURL else ("https://{}".format(hostname) if hostname else "")

# The primary Service a CNPG Cluster publishes. -ro and -r also exist; this app
# writes on every request that matters, so it always talks to the primary.
dbHost = config.dbHost if config.dbHost else "{}-rw.{}.svc".format(config.dbClusterName, config.namespace)

appSecretName = "{}-app".format(config.name)
dbSecretName = "{}-db".format(config.name)
configMapName = "{}-config".format(config.name)

renderExternalSecrets = config.secretsMode == "external"

# ── Labels ──────────────────────────────────────────────────────────────────
commonLabels = {
    "app.kubernetes.io/name": config.name
    "app.kubernetes.io/instance": config.name
    "app.kubernetes.io/component": "web"
    "app.kubernetes.io/part-of": config.name
    **config.labels
}

selectorLabels = {
    "app.kubernetes.io/name": config.name
    "app.kubernetes.io/instance": config.name
}
