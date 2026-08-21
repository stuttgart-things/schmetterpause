// Package main holds the Schmetterpause pipeline.
//
// The pipeline logic lives here rather than in a CI YAML: the same Go code
// runs in the same containers locally and in CI, which is why "task ci" is
// not an approximation of the pipeline but the pipeline itself.
//
// The generated parts of the module (internal/dagger, dagger.gen.go, go.mod)
// are produced by "dagger develop" on first use; they are not checked in.
package main

import (
	"context"
	"fmt"

	"dagger/schmetterpause/internal/dagger"
)

const (
	// goImage must carry at least the Go version from go.mod.
	goImage           = "golang:1.25-alpine"
	golangciLintImage = "golangci/golangci-lint:v2.6.0-alpine"
	postgresImage     = "postgres:17-alpine"
	runtimeImage      = "gcr.io/distroless/static-debian12:nonroot"
	toolingImage      = "alpine:3.24"

	// defaultArch is the target architecture when none is given.
	defaultArch = "amd64"

	// nonrootUID is the distroless image's user.
	nonrootUID = "65532:65532"

	// verifyDSN is the address at which the application reaches the Postgres
	// service during verify. It applies only inside the pipeline.
	verifyDSN = "postgres://schmetterpause:schmetterpause@db:5432/schmetterpause?sslmode=disable"
)

// Schmetterpause bundles the pipeline functions.
type Schmetterpause struct{}

// Lint checks formatting, the state of the generated files and static
// analysis.
//
// The generated-code check is the real point: it catches forgotten templ
// regeneration, which otherwise only shows up in the browser.
func (m *Schmetterpause) Lint(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
) (string, error) {
	vetOut, err := m.goBase(source).
		WithExec([]string{"sh", "-c", checksumCmd + " > /tmp/before"}).
		WithExec([]string{"go", "tool", "templ", "fmt", "."}).
		WithExec([]string{"go", "generate", "./..."}).
		WithExec([]string{"sh", "-c", checksumCmd + " > /tmp/after"}).
		WithExec([]string{"sh", "-c", generatedUpToDateCmd}).
		WithExec([]string{"go", "vet", "./..."}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("formatting, generated code and go vet: %w", err)
	}

	lintOut, err := dag.Container().
		From(golangciLintImage).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("schmetterpause-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("schmetterpause-go-build")).
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("schmetterpause-golangci")).
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"golangci-lint", "run", "--timeout", "5m"}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("golangci-lint: %w", err)
	}

	return vetOut + lintOut + "lint: passed\n", nil
}

// Test runs unit and repository tests against a fresh Postgres.
//
// Not part of Ci: the MVP plan defines the pipeline as lint, build, verify
// and keeps "task test" alongside as its own fast step. Whether that should
// change is tracked in issue #15. As a standalone call it still runs in the
// same environment as CI.
func (m *Schmetterpause) Test(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
) (string, error) {
	out, err := m.goBase(source).
		WithServiceBinding("db", m.Postgres()).
		// Set so the repository tests do not skip themselves.
		WithEnvVariable("SP_TEST_DATABASE_URL", verifyDSN).
		WithExec([]string{"go", "test", "-cover", "-count=1", "./..."}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("go test: %w", err)
	}
	return out, nil
}

// Binary builds the statically linked binary. CGO is off so it runs in the
// distroless image without libc.
func (m *Schmetterpause) Binary(
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
	// +optional
	// +default="amd64"
	goarch string,
) *dagger.File {
	return m.goBase(source).
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", goarch).
		WithExec([]string{
			"go", "build",
			"-trimpath",
			"-ldflags", "-s -w -X main.version=" + version,
			"-o", "/out/schmetterpause",
			"./cmd/schmetterpause",
		}).
		File("/out/schmetterpause")
}

// Image builds the runtime image. It matches the Dockerfile: the same image
// for Compose, Kubernetes and Azure Container Apps (invariant 1).
func (m *Schmetterpause) Image(
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
	// +optional
	// +default="amd64"
	goarch string,
) *dagger.Container {
	return dag.Container(dagger.ContainerOpts{Platform: dagger.Platform("linux/" + goarch)}).
		From(runtimeImage).
		WithFile("/usr/local/bin/schmetterpause", m.Binary(source, version, goarch)).
		WithUser(nonrootUID).
		WithExposedPort(8080).
		WithEntrypoint([]string{"/usr/local/bin/schmetterpause"}).
		WithDefaultArgs([]string{"serve"})
}

// Build emits the runtime image as an OCI tarball.
func (m *Schmetterpause) Build(
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
	// +optional
	// +default="amd64"
	goarch string,
) *dagger.File {
	return m.Image(source, version, goarch).AsTarball()
}

// Postgres starts a fresh database as a Dagger service.
func (m *Schmetterpause) Postgres() *dagger.Service {
	return dag.Container().
		From(postgresImage).
		WithEnvVariable("POSTGRES_USER", "schmetterpause").
		WithEnvVariable("POSTGRES_PASSWORD", "schmetterpause").
		WithEnvVariable("POSTGRES_DB", "schmetterpause").
		WithEnvVariable("PGDATA", "/var/lib/postgresql/data/pgdata").
		WithExposedPort(5432).
		AsService(dagger.ContainerAsServiceOpts{UseEntrypoint: true})
}

// Verify checks the built image against a fresh Postgres.
//
// The step deliberately tests the image and not the source: it catches faults
// that "go test" never sees — migrations missing from the image, broken env
// defaults, templates or assets that did not get embedded.
//
// The end-to-end path from the MVP plan (create two players, record a match,
// confirm it, check the ranking) arrives once AP4 and AP5 land. Tracked in
// issue #16.
func (m *Schmetterpause) Verify(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
) (string, error) {
	app := m.Image(source, version, defaultArch).
		WithEnvVariable("SP_DATABASE_URL", verifyDSN).
		WithEnvVariable("SP_LOG_LEVEL", "debug").
		WithServiceBinding("db", m.Postgres()).
		AsService(dagger.ContainerAsServiceOpts{UseEntrypoint: true})

	out, err := dag.Container().
		From(toolingImage).
		WithExec([]string{"apk", "add", "--no-cache", "curl"}).
		WithServiceBinding("app", app).
		// Forces re-evaluation when the version changes.
		WithEnvVariable("SP_VERIFY_VERSION", version).
		WithExec([]string{"sh", "-c", verifyScript}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("verify against the built image: %w", err)
	}
	return out, nil
}

// Ci runs lint, build and verify in that order. Verify depends on the build
// artefact, not on the source; if a step fails, the following ones are
// skipped.
func (m *Schmetterpause) Ci(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
) (string, error) {
	lintOut, err := m.Lint(ctx, source)
	if err != nil {
		return "", err
	}

	if _, err := m.Image(source, version, defaultArch).Sync(ctx); err != nil {
		return "", fmt.Errorf("build: %w", err)
	}

	verifyOut, err := m.Verify(ctx, source, version)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("== lint ==\n%s\n== build ==\nimage built (version %s)\n\n== verify ==\n%s",
		lintOut, version, verifyOut), nil
}

// goBase is the shared build container with warm module and build caches.
func (m *Schmetterpause) goBase(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From(goImage).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("schmetterpause-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("schmetterpause-go-build")).
		// Without .git in the context, -buildvcs would otherwise fail.
		WithEnvVariable("GOFLAGS", "-buildvcs=false").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"})
}

// checksumCmd captures the state of all templ sources and generated files.
const checksumCmd = `find . -path ./dagger -prune -o \( -name '*.templ' -o -name '*_templ.go' \) -print0 | sort -z | xargs -0 sha256sum`

// generatedUpToDateCmd fails when templ fmt or go generate changed
// anything.
const generatedUpToDateCmd = `
if ! diff -u /tmp/before /tmp/after; then
	echo
	echo "Generated files are out of date."
	echo "Run 'task generate' locally and commit the changes."
	exit 1
fi
echo "generated code: up to date"
`

// verifyScript is the end-to-end path against the running image.
const verifyScript = `
set -eu

echo "== /healthz =="
i=0
while [ $i -lt 60 ]; do
	curl -fsS http://app:8080/healthz >/dev/null 2>&1 && break
	i=$((i + 1))
	sleep 1
done
curl -fsS http://app:8080/healthz

echo "== /readyz: migrations applied, database reachable =="
i=0
while [ $i -lt 60 ]; do
	curl -fsS http://app:8080/readyz >/dev/null 2>&1 && break
	i=$((i + 1))
	sleep 1
done
if ! curl -fsS http://app:8080/readyz; then
	echo "readyz never became ready"
	curl -sS -i http://app:8080/readyz || true
	exit 1
fi

echo "== start page: templates are embedded =="
curl -fsS http://app:8080/ | grep -q "Schmetterpause" || {
	echo "start page missing expected content"
	exit 1
}

echo "== static assets: HTMX is embedded =="
size=$(curl -fsS http://app:8080/static/js/htmx.min.js | wc -c)
[ "$size" -gt 1000 ] || {
	echo "htmx.min.js missing or empty ($size bytes)"
	exit 1
}

echo "== HTMX fragment: handler, repository and database =="
curl -fsS http://app:8080/fragments/status | grep -q ">erreichbar<" || {
	echo "status fragment does not report the database as reachable"
	curl -sS http://app:8080/fragments/status || true
	exit 1
}

echo
echo "verify: all checks passed"
`
