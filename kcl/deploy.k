"""
Deployment for schmetterpause: one pod, a migration that runs before it, and no
writable path anywhere.
"""

import labels

config = labels.config

# The database URL, from the Secret that also carries CNPG's owner credentials.
_dbEnv = {
    name: "SP_DATABASE_URL"
    valueFrom: {
        secretKeyRef: {
            name: labels.dbSecretName
            key: "SP_DATABASE_URL"
        }
    }
}

# Applied to both containers. readOnlyRootFilesystem needs no emptyDir here:
# templates, static assets and migrations are embedded in the binary and
# nothing in the application writes to disk, so there is no /tmp to mount.
_containerSecurity = {
    readOnlyRootFilesystem: True
    runAsNonRoot: True
    allowPrivilegeEscalation: False
    capabilities: {
        drop: ["ALL"]
    }
}

deployment = {
    apiVersion: "apps/v1"
    kind: "Deployment"
    metadata: {
        name: config.name
        namespace: config.namespace
        labels: labels.commonLabels
        if config.annotations:
            annotations: config.annotations
    }
    spec: {
        replicas: config.replicas
        selector: {
            matchLabels: labels.selectorLabels
        }
        # Recreate, not RollingUpdate. A rolling update would briefly run the
        # new pod's "migrate up" while the old pod still serves the old schema.
        # At one replica the only thing a surge buys is that overlap.
        strategy: {
            type: "Recreate"
        }
        template: {
            metadata: {
                labels: labels.commonLabels
            }
            spec: {
                serviceAccountName: config.name
                automountServiceAccountToken: False
                # Migrations run here rather than in the server, so the schema
                # change is a separate container with a separate environment.
                # It gets the database Secret and nothing else: "migrate" calls
                # config.Load but not ValidateForServe, so it needs no session
                # key to start (cmd/schmetterpause/main.go).
                if config.migrateInitContainer:
                    initContainers: [
                        {
                            name: "migrate"
                            image: config.image
                            imagePullPolicy: config.imagePullPolicy
                            args: ["migrate", "up"]
                            env: [_dbEnv]
                            resources: {
                                requests: {
                                    cpu: config.cpuRequest
                                    memory: config.memoryRequest
                                }
                                limits: {
                                    cpu: config.cpuLimit
                                    memory: config.memoryLimit
                                }
                            }
                            securityContext: _containerSecurity
                        }
                    ]
                containers: [
                    {
                        name: config.name
                        image: config.image
                        imagePullPolicy: config.imagePullPolicy
                        args: ["serve"]
                        ports: [
                            {
                                name: "http"
                                containerPort: config.containerPort
                                protocol: "TCP"
                            }
                        ]
                        env: [_dbEnv]
                        envFrom: [
                            {
                                configMapRef: {
                                    name: labels.configMapName
                                }
                            }
                            {
                                secretRef: {
                                    name: labels.appSecretName
                                }
                            }
                        ]
                        resources: {
                            requests: {
                                cpu: config.cpuRequest
                                memory: config.memoryRequest
                            }
                            limits: {
                                cpu: config.cpuLimit
                                memory: config.memoryLimit
                            }
                        }
                        # The split is the point, and it is why these are
                        # httpGet rather than tcpSocket: /healthz ignores the
                        # database, /readyz checks it. A database outage then
                        # takes the pod out of the endpoint set instead of
                        # restarting it in a loop that cannot help.
                        livenessProbe: {
                            httpGet: {
                                path: "/healthz"
                                port: "http"
                            }
                            initialDelaySeconds: 5
                            periodSeconds: 10
                            timeoutSeconds: 3
                            failureThreshold: 3
                        }
                        readinessProbe: {
                            httpGet: {
                                path: "/readyz"
                                port: "http"
                            }
                            initialDelaySeconds: 2
                            periodSeconds: 5
                            timeoutSeconds: 3
                            failureThreshold: 3
                        }
                        securityContext: _containerSecurity
                    }
                ]
                securityContext: {
                    runAsNonRoot: True
                    runAsUser: 65532
                    runAsGroup: 65532
                    fsGroup: 65532
                    seccompProfile: {
                        type: "RuntimeDefault"
                    }
                }
                # SP_SHUTDOWN_TIMEOUT defaults to 15s, so give the kubelet a
                # little more than that before it stops asking politely.
                terminationGracePeriodSeconds: 20
            }
        }
    }
}
