"""
ServiceAccount for schmetterpause.

It talks to nothing in the cluster API, so it carries no RBAC. It exists so the
pod does not run as "default", which is the account every other workload in the
namespace also gets.
"""

import labels

config = labels.config

serviceAccount = {
    apiVersion: "v1"
    kind: "ServiceAccount"
    metadata: {
        name: config.name
        namespace: config.namespace
        labels: labels.commonLabels
    }
    automountServiceAccountToken: False
}
