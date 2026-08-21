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
	"strings"

	"dagger/schmetterpause/internal/dagger"
)

const (
	// goImage must carry at least the Go version from go.mod, and must match
	// the builder stage in the Dockerfile — otherwise the pipeline verifies an
	// image built by a different compiler than the one the Dockerfile uses.
	goImage           = "golang:1.27-alpine"
	golangciLintImage = "golangci/golangci-lint:v2.6.0-alpine"
	postgresImage     = "postgres:18-alpine"
	runtimeImage      = "gcr.io/distroless/static-debian12:nonroot"
	toolingImage      = "alpine:3.24"

	// defaultArch is the target architecture when none is given.
	defaultArch = "amd64"

	// nonrootUID is the distroless image's user.
	nonrootUID = "65532:65532"

	// ephemeralRegistry is ttl.sh: no account, no token, and images expire on
	// their own. For handing a build to someone before it is merged.
	//
	// It is public and anonymous — anyone who guesses the name can pull, and
	// anyone can push over it. That is acceptable for what goes in: a static
	// binary plus embedded CSS and HTMX, no credentials and no data. If the
	// image ever gains something that should not be world-readable, ttl.sh
	// stops being an option. See issue #20.
	ephemeralRegistry = "ttl.sh"

	// releaseRegistry holds the artefacts meant to last. Same account as the
	// repository, so GITHUB_TOKEN with packages:write is all it needs.
	releaseRegistry = "ghcr.io"

	// releasePlatforms are the architectures a release covers. Compose and
	// Azure Container Apps are amd64; arm64 is there so the image also runs
	// on an Apple-silicon laptop and on arm nodes.
	releasePlatformAmd64 = "amd64"
	releasePlatformArm64 = "arm64"

	// verifyDSN is the address at which the application reaches the Postgres
	// service during verify. It applies only inside the pipeline.
	verifyDSN = "postgres://schmetterpause:schmetterpause@db:5432/schmetterpause?sslmode=disable"

	// verifySessionKey signs cookies inside the pipeline only. It never
	// leaves the ephemeral verify containers.
	verifySessionKey = "pipeline-only-session-key-0123456789abcdef"
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
// The session check covers the Definition of Done of AP2 against the built
// image: join, keep the cookie, be recognised on the next request.
//
// The rest of the end-to-end path from the MVP plan (record a match, confirm
// it, check the ranking) arrives once AP4 and AP5 land. Tracked in issue #16.
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
		WithEnvVariable("SP_SESSION_KEY", verifySessionKey).
		// No TLS in front of the service, so a Secure cookie would never
		// come back and the session check below would pass for the wrong
		// reason.
		WithEnvVariable("SP_COOKIE_SECURE", "false").
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

// Publish pushes the runtime image to ttl.sh so it can be handed to somebody
// for a look.
//
// Needs no account and no token. The tag *is* the lifetime: ttl.sh deletes
// the image once it expires, so nothing has to be cleaned up afterwards.
// Accepted values run from a few minutes up to 24h.
//
// Returns the reference the image was pushed to.
func (m *Schmetterpause) Publish(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
	// +optional
	// +default="1h"
	ttl string,
	// +optional
	// +default="amd64"
	goarch string,
) (string, error) {
	ref := fmt.Sprintf("%s/schmetterpause-%s:%s", ephemeralRegistry, imageNameSafe(version), ttl)

	published, err := m.Image(source, version, goarch).Publish(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("publish to %s: %w", ephemeralRegistry, err)
	}
	return published, nil
}

// Release pushes the runtime image to the registry that keeps it, tagged with
// version and covering both architectures in one manifest.
//
// registryToken is a token with write access to the package — in Actions the
// job's GITHUB_TOKEN with packages:write is enough.
//
// Returns the reference the image was pushed to.
func (m *Schmetterpause) Release(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
	// The version to tag. No default on purpose: an artefact tagged "dev"
	// in a registry that keeps things is worse than no artefact.
	version string,
	// Token with write access to the package.
	registryToken *dagger.Secret,
	// The repository to push to. A "+default" pragma takes a literal, so
	// this string is the single place the release repository is named.
	// +optional
	// +default="ghcr.io/stuttgart-things/schmetterpause"
	image string,
	// The account the token belongs to.
	username string,
	// +optional
	// Also move the "latest" tag to this image.
	latest bool,
) (string, error) {
	if version == "" || version == "dev" {
		return "", fmt.Errorf("release needs a real version, got %q", version)
	}

	variants := []*dagger.Container{
		m.Image(source, version, releasePlatformAmd64),
		m.Image(source, version, releasePlatformArm64),
	}

	authed := dag.Container().
		WithRegistryAuth(releaseRegistry, username, registryToken)

	published, err := authed.Publish(ctx, image+":"+imageTagSafe(version),
		dagger.ContainerPublishOpts{PlatformVariants: variants})
	if err != nil {
		return "", fmt.Errorf("publish %s to %s: %w", version, releaseRegistry, err)
	}

	if latest {
		if _, err := authed.Publish(ctx, image+":latest",
			dagger.ContainerPublishOpts{PlatformVariants: variants}); err != nil {
			return "", fmt.Errorf("move the latest tag: %w", err)
		}
	}

	return published, nil
}

// imageNameSafe turns a version into something usable as a repository name.
// "git describe" happily produces "v0.2.0-3-gab12cd-dirty", and a registry
// accepts only lowercase alphanumerics with single separators.
func imageNameSafe(version string) string {
	return trimSeparators(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, version))
}

// imageTagSafe keeps a version usable as a tag. Tags allow more than
// repository names do — upper case, dots and underscores are fine — so a
// "v1.2.3" survives unchanged.
func imageTagSafe(version string) string {
	return trimSeparators(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, version))
}

func trimSeparators(s string) string {
	s = strings.Trim(s, "-._")
	if s == "" {
		return "unversioned"
	}
	return s
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

echo "== session: a browser is recognised again =="
cookies=$(mktemp)

curl -fsS -c "$cookies" -X POST http://app:8080/players \
	--data-urlencode "display_name=Verify Anna" | grep -q "Verify Anna" || {
	echo "joining did not name the player back"
	exit 1
}

grep -q schmetterpause_session "$cookies" || {
	echo "no session cookie was set"
	exit 1
}

# The point of AP2: the same browser maps to the same player on a later
# request, through nothing but the cookie it kept.
curl -fsS -b "$cookies" http://app:8080/ | grep -q "Angemeldet als" || {
	echo "the returning request was not recognised"
	exit 1
}

echo "== match entry: an impossible result is refused, a real one is stored =="
second=$(mktemp)
curl -fsS -c "$second" -X POST http://app:8080/players \
	--data-urlencode "display_name=Verify Bodo" >/dev/null

# The opponent id is only reachable through the rendered picker, which is
# also the point: it checks that the form offers a usable opponent at all.
opponent=$(curl -fsS -b "$cookies" http://app:8080/fragments/match \
	| tr "<" "\n" | grep "option value=\"" | grep -v "value=\"\"" | head -1 \
	| sed "s/.*value=\"\([^\"]*\)\".*/\1/")
[ -n "$opponent" ] || {
	echo "the entry form offered no opponent"
	exit 1
}

# 11:10 is one clear point, not two. The Definition of Done is that the
# refusal says why, so the status alone is not enough to check.
refusal=$(mktemp)
code=$(curl -sS -b "$cookies" -o "$refusal" -w "%{http_code}" -X POST http://app:8080/matches \
	--data "opponent_id=$opponent&best_of=3&points_to_win=11&set_home_1=11&set_away_1=10")
[ "$code" = "422" ] || {
	echo "an impossible result was answered with $code, expected 422"
	exit 1
}
grep -q "zwei Punkte Vorsprung" "$refusal" || {
	echo "the refusal does not say why"
	cat "$refusal"
	exit 1
}

curl -fsS -b "$cookies" -X POST http://app:8080/matches \
	--data "opponent_id=$opponent&best_of=3&points_to_win=11&set_home_1=11&set_away_1=9&set_home_2=12&set_away_2=10" \
	| grep -q "bestätigen" || {
	echo "a valid result was not stored as pending confirmation"
	exit 1
}

echo
echo "verify: all checks passed"
`
