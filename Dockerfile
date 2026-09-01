# syntax=docker/dockerfile:1

# Multi-stage build producing a distroless runtime image.
#
# One binary, one image (invariant 1): this image runs unchanged in Docker
# Compose, Kubernetes and Azure Container Apps. It contains no config file;
# everything environment-dependent arrives through SP_* variables
# (invariant 2). Templates, static assets and migrations are embedded in the
# binary — the image needs no volume.

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

WORKDIR /src

# Dependencies first: this layer stays valid as long as go.mod and go.sum do
# not change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
# When the commit was made, not when this ran. A build clock would give two
# images from one commit different contents, and "is the same thing running
# here and there" is exactly what this is for.
ARG COMMIT_TIME=
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commitTime=${COMMIT_TIME}" \
        -o /out/schmetterpause \
        ./cmd/schmetterpause

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/schmetterpause /usr/local/bin/schmetterpause

USER nonroot:nonroot

# Metadata only. The actual port comes from SP_HTTP_ADDR; the default in code
# is ":8080".
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/schmetterpause"]
CMD ["serve"]
