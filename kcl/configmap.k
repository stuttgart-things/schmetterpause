"""
The uncritical half of the configuration.

What is here is readable by anyone who can read the namespace, which is exactly
why the session key, the kiosk token and the database URL are not. Those live
in Secrets — see externalsecret.k.

SP_COOKIE_SECURE is deliberately absent. It defaults to true in the code so a
forgotten setting fails closed; compose.yaml is what turns it off, for local
HTTP. Setting it here could only ever weaken it.
"""

import labels

config = labels.config

configMap = {
    apiVersion: "v1"
    kind: "ConfigMap"
    metadata: {
        name: labels.configMapName
        namespace: config.namespace
        labels: labels.commonLabels
    }
    data: {
        SP_LOG_LEVEL: config.logLevel
        SP_HTTP_ADDR: ":{}".format(config.containerPort)
        # False, always. Migrations run as an initContainer instead, so the
        # step that changes the schema is a separate container with a separate
        # environment rather than something the web server does on the way up.
        SP_AUTO_MIGRATE: "false"
        if labels.publicBaseURL:
            SP_PUBLIC_BASE_URL: labels.publicBaseURL
        if config.bootstrapAdmin:
            SP_BOOTSTRAP_ADMIN: config.bootstrapAdmin
        **config.extraEnvVars
    }
}
