# syntax=docker/dockerfile:1

# Multi-Stage-Build zu einem distroless-Laufzeitimage.
#
# Ein Binary, ein Image (Invariante 1): dieses Image laeuft unveraendert in
# Docker Compose, Kubernetes und Azure Container Apps. Es enthaelt keine
# Config-Datei; alles Umgebungsabhaengige kommt ueber SP_*-Variablen
# (Invariante 2). Templates, statische Assets und Migrations sind ins Binary
# eingebettet — das Image braucht kein Volume.

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

WORKDIR /src

# Abhaengigkeiten zuerst: die Schicht bleibt gueltig, solange sich go.mod und
# go.sum nicht aendern.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o /out/schmetterpause \
        ./cmd/schmetterpause

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/schmetterpause /usr/local/bin/schmetterpause

USER nonroot:nonroot

# Nur Metadaten. Der tatsaechliche Port kommt aus SP_HTTP_ADDR; der Default im
# Code ist ":8080".
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/schmetterpause"]
CMD ["serve"]
