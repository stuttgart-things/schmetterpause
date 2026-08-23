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
	golangciLintImage = "golangci/golangci-lint:v2.13.1-alpine"
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

	// verifyKioskToken unlocks the kiosk inside the pipeline only, for the
	// same reason and with the same lifetime.
	verifyKioskToken = "pipeline-only-kiosk-token"
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
// Part of Ci, and second in it: a failing unit test should not have to wait
// for an image build to be reported. It is also still callable on its own,
// which is what "task test" does.
//
// Verify does not make it redundant. Verify drives the built image through
// one end-to-end path and can only see what that path touches; these tests
// reach the cases a browser cannot reasonably be walked through — a rejected
// result for every rejection kind, a rollback, a rating that moves by zero.
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
		// -p 1: the repository and scoring suites share this database and
		// empty it between tests, so running their packages concurrently has
		// them migrating and truncating over each other.
		WithExec([]string{"go", "test", "-cover", "-count=1", "-p", "1", "./..."}).
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
		// The kiosk exists only where a token is set, so verify has to set
		// one to be able to check that it does.
		WithEnvVariable("SP_KIOSK_TOKEN", verifyKioskToken).
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

// Ci runs lint, test, build and verify in that order. The two source-level
// steps come first because they are the cheap ones; verify depends on the
// build artefact rather than on the source, so it comes last. If a step
// fails, the following ones are skipped.
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

	testOut, err := m.Test(ctx, source)
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

	return fmt.Sprintf(
		"== lint ==\n%s\n== test ==\n%s\n== build ==\nimage built (version %s)\n\n== verify ==\n%s",
		lintOut, testOut, version, verifyOut), nil
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

echo "== fonts: embedded and served =="
size=$(curl -fsS http://app:8080/static/fonts/space-grotesk-latin.woff2 | wc -c)
[ "$size" -gt 10000 ] || {
	echo "the display font is missing or empty ($size bytes)"
	exit 1
}

echo "== mark: the icons are embedded and are really images =="
icon=$(mktemp)
for path in /static/img/mark-32.png /static/img/mark-180.png /static/img/mascot.png /favicon.ico; do
	curl -fsS "http://app:8080$path" > "$icon" || {
		echo "$path is not served"
		exit 1
	}
	# A rendered error page would come back with a 200 on it, so the check has
	# to look at the bytes rather than at the status. Reading the file rather
	# than piping into head: a pipe that closes early makes curl report a write
	# failure, which would hide a real one.
	case "$(od -An -c -N 8 "$icon" | tr -d " \n")" in
		*PNG*) ;;
		*)
			echo "$path is not a PNG"
			exit 1
			;;
	esac
done

echo "== mark: the top bar shows it =="
curl -fsS http://app:8080/ | grep -q "class=\"brand-logo\"" || {
	echo "the mark is missing from the top bar"
	exit 1
}

echo "== HTMX fragment: handler, repository and database =="
status=$(mktemp)
curl -fsS http://app:8080/fragments/status > "$status"
grep -q ">erreichbar<" "$status" || {
	echo "status fragment does not report the database as reachable"
	cat "$status"
	exit 1
}

# The probes are values in the page now, not two links a player has to click
# and read. Both states have to be there, not just the word.
for want in ">healthz<" ">ok<" ">readyz<" ">bereit<"; do
	grep -q "$want" "$status" || {
		echo "the status box does not state $want"
		cat "$status"
		exit 1
	}
done

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
# request, through nothing but the cookie it kept. Being recognised shows in
# the top bar, and it is rendered server-side — a name that only turns up
# once a fragment has loaded would pass a test and flicker for a player.
home=$(mktemp)
curl -fsS -b "$cookies" http://app:8080/ > "$home"
grep -q "class=\"whoami-name\"" "$home" || {
	echo "the top bar does not say who this is"
	exit 1
}
grep -q ">Verify Anna</a>" "$home" || {
	echo "the returning request was not recognised"
	exit 1
}

echo "== top bar: the badge counts what waits =="
curl -fsS -b "$cookies" http://app:8080/fragments/whoami | grep -q "whoami-badge" && {
	echo "a badge turned up with nothing waiting"
	exit 1
}

echo "== set rows: as many as the mode can have, and no more =="
form=$(mktemp)
curl -fsS -b "$cookies" "http://app:8080/fragments/sets?sets_prefix=entry&best_of=3&points_to_win=11" > "$form"
grep -q "set_home_3" "$form" || {
	echo "best-of-three is missing its third set"
	exit 1
}
# if rather than "grep && { }": under set -e a grep that finds nothing is the
# last command of an AND-OR list, and whether that ends the script is a
# question about the shell rather than about the page.
if grep -q "set_home_4" "$form"; then
	echo "best-of-three offers a fourth set"
	exit 1
fi
# No maximum on a score box: at eleven, 12:10 and 13:11 are ordinary results,
# so the rule is written out beside them instead. The slider next to the box
# does carry one, hence the type="number" in the pattern — one element per
# line first, so the two cannot be confused.
if tr "<" "\n" < "$form" | grep "type=\"number\"" | grep -q "max="; then
	echo "a score box carries a maximum, which would reject 12:10"
	exit 1
fi
grep -q "10:10" "$form" || {
	echo "the deuce rule is not stated where the scores are typed"
	exit 1
}
# Two serves each to eleven. The other mode says five, which is why the rule
# is written from the mode rather than as a constant.
grep -q "je 2 Punkten" "$form" || {
	echo "the service rule is not stated where the scores are typed"
	exit 1
}

echo "== the mascot greets from the top of the page =="
top=$(mktemp)
curl -fsS -b "$cookies" http://app:8080/ > "$top"
grep -q "page-mascot" "$top" || {
	echo "the start page has no mascot"
	exit 1
}

echo "== score columns: each one says whose it is =="
sides=$(mktemp)
curl -fsS -b "$cookies" "http://app:8080/fragments/sets?sets_prefix=entry&best_of=3&points_to_win=11" > "$sides"
grep -q ">Du<" "$sides" || {
	echo "the left score column is not named"
	cat "$sides"
	exit 1
}
grep -q ">Gegner<" "$sides" || {
	echo "the right score column is not named"
	cat "$sides"
	exit 1
}

# Every box comes up with a zero in it, so the slider beside it has something
# to point at and nobody types a zero for a set lost to nil.
if tr "<" "\n" < "$sides" | grep "type=\"number\"" | grep -q "value=\"\""; then
	echo "a score box came up empty"
	exit 1
fi

echo "== the script is not held for an hour =="
# A deployment replaces the HTML at once and a cached script not at all, and
# the old script against new markup is a feature that is drawn and does not
# work. Seen once, hence the check.
cache=$(curl -fsS -o /dev/null -D - http://app:8080/static/js/app.js | tr -d "\r" \
	| grep -i "^cache-control:" | cut -d" " -f2-)
[ "$cache" = "no-cache" ] || {
	echo "app.js is served with Cache-Control: $cache, expected no-cache"
	exit 1
}

echo "== a rejected form reaches the page =="
size=$(curl -fsS http://app:8080/static/js/app.js | wc -c)
[ "$size" -gt 100 ] || {
	# Without it HTMX drops the 422 that carries the reason, and a wrong
	# score looks like a broken button.
	echo "the swap-on-422 script is missing or empty ($size bytes)"
	exit 1
}

echo "== ranking: nobody is on a position before anybody has played =="
fresh=$(mktemp)
curl -fsS -b "$cookies" http://app:8080/fragments/standings > "$fresh"
grep -q "rank-none" "$fresh" || {
	echo "a player without a confirmed match was given a position"
	cat "$fresh"
	exit 1
}
if grep -q "rank rank-1" "$fresh"; then
	echo "first place was handed out before a single match was confirmed"
	exit 1
fi

echo "== ranking: the table scrolls, the page does not =="
curl -fsS http://app:8080/ | grep -q "class=\"table-scroll\" tabindex=\"0\"" || {
	echo "the ranking is not in a focusable scroll box"
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

# Anna entered one, so it now waits on Bodo and the badge says so.
curl -fsS -b "$second" http://app:8080/fragments/whoami | grep -q "whoami-badge" || {
	echo "the badge does not count the result waiting on the opponent"
	exit 1
}

echo "== confirmation: only the opponent settles a result =="
mid=$(curl -fsS -b "$second" http://app:8080/fragments/pending \
	| tr "<" "\n" | grep "li id=\"pending-" | head -1 \
	| sed "s/.*pending-\([0-9a-f-]*\)\".*/\1/")
[ -n "$mid" ] || {
	echo "the opponent was shown nothing to confirm"
	exit 1
}

# Anna entered it, so Anna cannot confirm it. A result confirmed by whoever
# reported it is not confirmed at all, which is the point of the step.
code=$(curl -sS -o /dev/null -w "%{http_code}" -b "$cookies" \
	-X POST "http://app:8080/matches/$mid/confirm")
[ "$code" = "403" ] || {
	echo "the reporter was allowed to confirm their own result: $code"
	exit 1
}

curl -fsS -b "$second" -X POST "http://app:8080/matches/$mid/confirm" \
	| grep -q "Bestätigt" || {
	echo "confirming did not settle the match"
	exit 1
}

# 11:9 and 12:10 from equal ratings is +8, so the ranking has to have moved.
curl -fsS -b "$second" http://app:8080/fragments/standings > /tmp/standings
grep -q "1008" /tmp/standings || {
	echo "the rating did not move after the confirmation"
	cat /tmp/standings
	exit 1
}
# One played, one won: the tally counts confirmed matches, and there is
# exactly one of those.
grep -q "1:0" /tmp/standings || {
	echo "the ranking does not show the win/loss record"
	cat /tmp/standings
	exit 1
}

echo "== correction: a contested result reaches a rating without SQL =="
second_match=$(mktemp)
curl -fsS -b "$cookies" -X POST http://app:8080/matches \
	--data "opponent_id=$opponent&best_of=3&points_to_win=11&set_home_1=11&set_away_1=8&set_home_2=11&set_away_2=6" \
	> /dev/null

mid2=$(curl -fsS -b "$second" http://app:8080/fragments/pending \
	| tr "<" "\n" | grep "li id=\"pending-" | grep -v "$mid" | head -1 \
	| sed "s/.*pending-\([0-9a-f-]*\)\".*/\1/")
[ -n "$mid2" ] || {
	echo "the second match never reached the opponent"
	exit 1
}

# Bodo says it was the other way round. The form has to come back filled in
# with what Anna claimed, seen from his side.
curl -fsS -b "$second" -X POST "http://app:8080/matches/$mid2/dispute" > "$second_match"
grep -q "Strittig" "$second_match" || {
	echo "disputing did not offer a correction"
	cat "$second_match"
	exit 1
}
grep -q "set_home_1\" value=\"8\"" "$second_match" || {
	echo "the correction form did not open on the reported result"
	cat "$second_match"
	exit 1
}

# A contested match must survive a reload, or the correction is unreachable.
curl -fsS -b "$cookies" http://app:8080/fragments/pending | grep -q "/correct" || {
	echo "the reporter cannot reach the correction after a reload"
	exit 1
}

# The corrected result is held to the same rules as a fresh one.
code=$(curl -sS -b "$second" -o /dev/null -w "%{http_code}" \
	-X POST "http://app:8080/matches/$mid2/correct" \
	--data "best_of=3&points_to_win=11&set_home_1=11&set_away_1=10&set_home_2=11&set_away_2=9")
[ "$code" = "422" ] || {
	echo "an impossible correction was answered with $code, expected 422"
	exit 1
}

curl -fsS -b "$second" -X POST "http://app:8080/matches/$mid2/correct" \
	--data "best_of=3&points_to_win=11&set_home_1=11&set_away_1=8&set_home_2=11&set_away_2=6" \
	| grep -q "Korrigiert" || {
	echo "the correction was not accepted"
	exit 1
}

# Whoever corrected became the reporter, so the other one confirms.
curl -fsS -b "$cookies" -X POST "http://app:8080/matches/$mid2/confirm" \
	| grep -q "Bestätigt" || {
	echo "the corrected result could not be confirmed"
	exit 1
}

# Anna won the first, Bodo the corrected second: back to level, and each with
# one win and one loss.
curl -fsS -b "$cookies" http://app:8080/fragments/standings > /tmp/standings2
grep -q "1:1" /tmp/standings2 || {
	echo "the corrected result did not reach the ranking"
	cat /tmp/standings2
	exit 1
}

echo "== kiosk: one machine enters for everybody =="
# Locked until the token is shown, and the token is swapped for a cookie.
code=$(curl -sS -o /dev/null -w "%{http_code}" http://app:8080/kiosk)
[ "$code" = "403" ] || {
	echo "the kiosk answered $code without a token, expected 403"
	exit 1
}

kiosk=$(mktemp)
curl -fsS -c "$kiosk" -o /dev/null "http://app:8080/kiosk?token=pipeline-only-kiosk-token"
grep -q schmetterpause_kiosk "$kiosk" || {
	echo "the token did not unlock the kiosk"
	exit 1
}

# A player created here must not sign the kiosk in as them.
before=$(grep -c schmetterpause_session "$kiosk" || true)
curl -fsS -b "$kiosk" -c "$kiosk" -X POST http://app:8080/kiosk/players \
	--data-urlencode "display_name=Kiosk Cara" > /dev/null
after=$(grep -c schmetterpause_session "$kiosk" || true)
[ "$before" = "$after" ] || {
	echo "creating a player at the kiosk signed the kiosk in as them"
	exit 1
}
curl -fsS -b "$kiosk" -X POST http://app:8080/kiosk/players \
	--data-urlencode "display_name=Kiosk Dirk" > /dev/null

kiosk_page=$(mktemp)
curl -fsS -b "$kiosk" http://app:8080/kiosk > "$kiosk_page"
grep -q "page-mascot" "$kiosk_page" || {
	echo "the kiosk has no mascot"
	exit 1
}
cara=$(tr "<" "\n" < "$kiosk_page" | grep "option value=\"" | grep "Kiosk Cara" \
	| sed "s/.*value=\"\([^\"]*\)\".*/\1/" | head -1)
dirk=$(tr "<" "\n" < "$kiosk_page" | grep "option value=\"" | grep "Kiosk Dirk" \
	| sed "s/.*value=\"\([^\"]*\)\".*/\1/" | head -1)

# Nobody plays themselves. Picking one side takes that name out of the other
# list, so the mistake is unavailable rather than punished after the fact.
picker=$(mktemp)
curl -fsS -b "$kiosk" -H "HX-Trigger: kiosk-home" \
	"http://app:8080/fragments/sets?sets_prefix=kiosk&home_id=$cara&best_of=3" > "$picker"
grep -q "hx-swap-oob" "$picker" || {
	echo "picking a player did not send the other list back"
	cat "$picker"
	exit 1
}
if grep -q "value=\"$cara\"" "$picker"; then
	echo "the opponent list still offers the player who is already on the other side"
	exit 1
fi
grep -q "Kiosk Dirk" "$picker" || {
	echo "the opponent list lost everybody else as well"
	exit 1
}
[ -n "$cara" ] && [ -n "$dirk" ] || {
	echo "the kiosk does not offer the players it created"
	exit 1
}

# Nobody is left to ask, so the result counts on submit.
curl -fsS -b "$kiosk" -X POST http://app:8080/kiosk/matches \
	--data "home_id=$cara&away_id=$dirk&best_of=3&points_to_win=11&set_home_1=11&set_away_1=9&set_home_2=11&set_away_2=7" \
	| grep -q "Kiosk Cara schlägt Kiosk Dirk 2:0" || {
	echo "the kiosk did not record the result"
	exit 1
}
curl -fsS -b "$kiosk" http://app:8080/kiosk | grep -q "1008" || {
	echo "the kiosk result did not reach the ranking"
	exit 1
}

echo "== profile: the rating history is drawn =="
pid=$(tr "<" "\n" < /tmp/standings | grep "a href=\"/players/" | head -1 \
	| sed "s|.*players/\([0-9a-f-]*\)\".*|\1|")
[ -n "$pid" ] || {
	echo "the ranking links to no profile"
	exit 1
}
curl -fsS -b "$cookies" "http://app:8080/players/$pid" > /tmp/profile
grep -q "<polyline" /tmp/profile || {
	echo "the profile shows no rating history"
	exit 1
}
# The baseline is not zero, so the range has to be stated next to the chart.
grep -q "Verlauf" /tmp/profile || {
	echo "the chart does not state its range"
	exit 1
}

echo "== QR sheet: the code points back at this host =="
sheet=$(mktemp)
curl -fsS -b "$cookies" http://app:8080/qr > "$sheet"

grep -q "http://app:8080/#match" "$sheet" || {
	echo "the sheet does not print the address it was reached at"
	cat "$sheet"
	exit 1
}

grep -q "class=\"sheet-mascot\"" "$sheet" || {
	echo "the sheet has no illustration"
	exit 1
}

# A path with more than a handful of runs. An empty or one-run path would
# still be valid SVG and would not be a QR code.
runs=$(tr "M" "\n" < "$sheet" | grep -c "^[0-9]* [0-9]*h[0-9]*v1h-[0-9]*z" || true)
[ "$runs" -gt 20 ] || {
	echo "the sheet drew $runs module runs, which is not a QR code"
	exit 1
}

# The anchor the code carries has to exist on the page it lands on.
curl -fsS -b "$cookies" http://app:8080/ | grep -q "id=\"match\"" || {
	echo "the start page has no #match for a scan to land on"
	exit 1
}

# Behind a TLS-terminating proxy the connection is plain and the printed
# address must not be.
curl -fsS -H "X-Forwarded-Proto: https" http://app:8080/qr \
	| grep -q "https://app:8080/#match" || {
	echo "the sheet ignored the forwarded scheme"
	exit 1
}

echo
echo "verify: all checks passed"
`
