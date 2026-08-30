"""
Gateway API routes.

Two of them, and the second one is not decoration.

The shared gateway serves the same wildcard on two listeners: HTTPS on 443 and
plain HTTP on 80. A route that names no sectionName attaches to both, so the
application would answer on plain HTTP as well — and there a browser never
returns the Secure session cookie that SP_COOKIE_SECURE asks for. A player
would join, be shown their recovery code once, and arrive at the next request
as a stranger. That is issue #70 rebuilt by a listener, and it would look like
an application bug.

So the application binds to the TLS listener by name, and the plain listener
gets a route of its own that only redirects.
"""

import labels

config = labels.config

_parentRef = lambda section: str -> {str:str} {
    {
        group: "gateway.networking.k8s.io"
        kind: "Gateway"
        name: config.gatewayName
        sectionName: section
        namespace: config.gatewayNamespace
    }
}

# group, kind and weight are admission-webhook defaults on parentRefs and
# backendRefs. Rendering them explicitly stops ArgoCD comparing a manifest
# without them against a live object with them and reporting OutOfSync on every
# reconcile — stuttgart-things/argocd#114.
#
# namespace is written unconditionally for a harsher reason. Left out, the
# parentRef resolves against the route's OWN namespace; if no Gateway is there,
# the route gets no status whatsoever — not Accepted=False, nothing — and the
# host answers 404 from the gateway with no trace of why. schema.k therefore
# refuses an empty gatewayNamespace outright, which is what makes this safe.
httproute = {
    apiVersion: "gateway.networking.k8s.io/v1"
    kind: "HTTPRoute"
    metadata: {
        name: config.name
        namespace: config.namespace
        labels: labels.commonLabels
        if config.annotations:
            annotations: config.annotations
    }
    spec: {
        parentRefs: [_parentRef(config.gatewaySectionNameHTTPS)]
        hostnames: [labels.hostname]
        rules: [
            {
                matches: [
                    {
                        path: {
                            type: "PathPrefix"
                            value: "/"
                        }
                    }
                ]
                backendRefs: [
                    {
                        group: ""
                        kind: "Service"
                        name: config.name
                        port: config.servicePort
                        weight: 1
                    }
                ]
            }
        ]
    }
} if config.httpRouteEnabled else None

# Port 80 answers with a redirect rather than with the application. 301 because
# the move is permanent: there is no version of this app that should be served
# over plain HTTP.
httprouteRedirect = {
    apiVersion: "gateway.networking.k8s.io/v1"
    kind: "HTTPRoute"
    metadata: {
        name: "{}-redirect".format(config.name)
        namespace: config.namespace
        labels: labels.commonLabels
        if config.annotations:
            annotations: config.annotations
    }
    spec: {
        parentRefs: [_parentRef(config.gatewaySectionNameHTTP)]
        hostnames: [labels.hostname]
        rules: [
            {
                filters: [
                    {
                        type: "RequestRedirect"
                        requestRedirect: {
                            scheme: "https"
                            statusCode: 301
                        }
                    }
                ]
            }
        ]
    }
} if config.httpRouteEnabled and config.httpRedirectEnabled else None
