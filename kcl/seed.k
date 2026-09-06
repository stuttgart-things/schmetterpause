"""
A Job that fills an empty database with a demo field.

For preview environments (issue #82): a fresh Schmetterpause is a join form and
an empty ranking, so a preview of a change to the standings, the match list or
a profile page shows nothing a reviewer can judge. The same fixture serves
screenshots.

Off unless a profile asks for it, and the binary refuses a database that
already has players — two guards, because this one writes rows into whatever
it is pointed at.
"""

import labels
import deploy

config = labels.config

seedJob = {
    apiVersion: "batch/v1"
    kind: "Job"
    metadata: {
        name: "{}-seed".format(config.name)
        namespace: config.namespace
        labels: labels.labelsFor("seed")
        annotations: {
            # PostSync rather than a plain resource, for two reasons that
            # happen to want the same thing.
            #
            # A Job's pod template is immutable, so a plain Job stops syncing
            # the moment the image tag changes — which in a preview is every
            # push. BeforeHookCreation deletes the previous one first, so the
            # new tag is a new Job rather than a rejected patch.
            #
            # And PostSync runs after the Deployment reports healthy, which is
            # after its migration initContainer has been through. Seeding a
            # database whose tables do not exist yet would otherwise be the
            # normal case on a first sync, recovered only by backoff.
            "argocd.argoproj.io/hook": "PostSync"
            "argocd.argoproj.io/hook-delete-policy": "BeforeHookCreation"
            # Spread last, so a caller may add annotations but setting either
            # key above is a render error rather than a hook that silently
            # stops being one. Same construction as the prune guard in
            # database.k.
            **config.annotations
        }
    }
    spec: {
        # Six attempts. The Job's own dependency is the database rather than
        # the schema — PostSync has already waited for that — and a CNPG
        # cluster that is still electing a primary is the case worth riding
        # out. Past that, failing is more useful than retrying forever.
        backoffLimit: 6
        template: {
            metadata: {
                labels: labels.labelsFor("seed")
            }
            spec: {
                restartPolicy: "OnFailure"
                serviceAccountName: config.name
                automountServiceAccountToken: False
                containers: [
                    {
                        name: "seed"
                        image: config.image
                        imagePullPolicy: config.imagePullPolicy
                        args: ["seed"]
                        # The database Secret and nothing else. "seed" calls
                        # config.Load but not ValidateForServe, exactly like
                        # "migrate up", so it needs no session key — and a Job
                        # that never holds one cannot leak one.
                        env: [
                            {
                                name: "SP_DATABASE_URL"
                                valueFrom: {
                                    secretKeyRef: {
                                        name: labels.dbSecretName
                                        key: "SP_DATABASE_URL"
                                    }
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
                        securityContext: deploy.containerSecurity
                    }
                ]
            }
        }
    }
} if config.seedEnabled else Undefined
