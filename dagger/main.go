// Package main enthaelt die Pipeline von Schmetterpause.
//
// Die Pipeline-Logik steht hier und nicht in einem CI-YAML: lokal und in der
// CI laeuft derselbe Go-Code in denselben Containern, "task ci" ist deshalb
// kein Naeherungswert an die Pipeline, sondern die Pipeline.
//
// Die generierten Teile des Moduls (internal/dagger, dagger.gen.go, go.mod)
// erzeugt "dagger develop" beim ersten Aufruf; sie sind nicht eingecheckt.
package main

import (
	"context"
	"fmt"

	"dagger/schmetterpause/internal/dagger"
)

const (
	// goImage muss mindestens die Go-Version aus go.mod mitbringen.
	goImage           = "golang:1.25-alpine"
	golangciLintImage = "golangci/golangci-lint:v2.6.0-alpine"
	postgresImage     = "postgres:17-alpine"
	runtimeImage      = "gcr.io/distroless/static-debian12:nonroot"
	toolingImage      = "alpine:3.21"

	// defaultArch ist die Zielarchitektur, wenn keine angegeben wird.
	defaultArch = "amd64"

	// nonrootUID ist der Benutzer des distroless-Images.
	nonrootUID = "65532:65532"

	// verifyDSN ist die Adresse, unter der die App im Verify-Schritt den
	// Postgres-Service erreicht. Sie gilt nur innerhalb der Pipeline.
	verifyDSN = "postgres://schmetterpause:schmetterpause@db:5432/schmetterpause?sslmode=disable"
)

// Schmetterpause buendelt die Pipeline-Funktionen.
type Schmetterpause struct{}

// Lint prueft Formatierung, den Stand der generierten Dateien und die
// statische Analyse.
//
// Der Generat-Check ist der eigentliche Punkt: er faengt vergessene
// templ-Regenerierung, die sonst erst im Browser auffaellt.
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
		return "", fmt.Errorf("formatierung, generat und go vet: %w", err)
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

	return vetOut + lintOut + "lint: bestanden\n", nil
}

// Test fuehrt Unit- und Repository-Tests gegen ein frisches Postgres aus.
//
// Nicht Teil von Ci: der MVP-Plan definiert die Pipeline als lint, build,
// verify und haelt "task test" als eigenen, schnellen Schritt daneben. Als
// eigener Aufruf laeuft er hier trotzdem in derselben Umgebung wie in der CI.
func (m *Schmetterpause) Test(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["**/.git", "build", ".task", "dagger/internal", "dagger/dagger.gen.go"]
	source *dagger.Directory,
) (string, error) {
	out, err := m.goBase(source).
		WithServiceBinding("db", m.Postgres()).
		// Gesetzt, damit die Repository-Tests nicht uebersprungen werden.
		WithEnvVariable("SP_TEST_DATABASE_URL", verifyDSN).
		WithExec([]string{"go", "test", "-cover", "-count=1", "./..."}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("go test: %w", err)
	}
	return out, nil
}

// Binary baut das statisch gelinkte Binary. CGO ist aus, damit es im
// distroless-Image ohne libc laeuft.
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

// Image baut das Laufzeitimage. Es entspricht dem Dockerfile: dasselbe Image
// fuer Compose, Kubernetes und Azure Container Apps (Invariante 1).
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

// Build gibt das Laufzeitimage als OCI-Tarball aus.
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

// Postgres startet eine frische Datenbank als Dagger-Service.
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

// Verify prueft das gebaute Image gegen ein frisches Postgres.
//
// Der Schritt testet bewusst das Image und nicht den Quellcode: Er faellt auf
// Fehler herein, die "go test" nie sieht — fehlende Migrations im Image,
// kaputte Env-Defaults, Templates oder Assets, die nicht eingebettet wurden.
//
// Der End-to-End-Pfad aus dem MVP-Plan (zwei Spieler anlegen, Match eintragen,
// bestaetigen, Rangliste pruefen) kommt hinzu, sobald AP4 und AP5 stehen.
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
		// Zwingt eine Neuauswertung, wenn sich die Version aendert.
		WithEnvVariable("SP_VERIFY_VERSION", version).
		WithExec([]string{"sh", "-c", verifyScript}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("verify gegen das gebaute image: %w", err)
	}
	return out, nil
}

// Ci fuehrt lint, build und verify in dieser Reihenfolge aus. Verify haengt am
// Build-Artefakt, nicht am Quellcode; scheitert ein Schritt, brechen die
// folgenden ab.
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

	return fmt.Sprintf("== lint ==\n%s\n== build ==\nimage gebaut (version %s)\n\n== verify ==\n%s",
		lintOut, version, verifyOut), nil
}

// goBase ist der gemeinsame Build-Container mit warmen Modul- und Build-Caches.
func (m *Schmetterpause) goBase(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From(goImage).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("schmetterpause-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("schmetterpause-go-build")).
		// Ohne .git im Kontext wuerde -buildvcs sonst fehlschlagen.
		WithEnvVariable("GOFLAGS", "-buildvcs=false").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"})
}

// checksumCmd bildet den Stand aller templ-Quellen und -Generate ab.
const checksumCmd = `find . -path ./dagger -prune -o \( -name '*.templ' -o -name '*_templ.go' \) -print0 | sort -z | xargs -0 sha256sum`

// generatedUpToDateCmd bricht ab, wenn templ fmt oder go generate etwas
// veraendert haben.
const generatedUpToDateCmd = `
if ! diff -u /tmp/before /tmp/after; then
	echo
	echo "Generierte Dateien sind nicht aktuell."
	echo "Lokal 'task generate' ausfuehren und die Aenderungen einchecken."
	exit 1
fi
echo "generat: aktuell"
`

// verifyScript ist der End-to-End-Pfad gegen das laufende Image.
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

echo "== /readyz: Migrations gelaufen, Datenbank erreichbar =="
i=0
while [ $i -lt 60 ]; do
	curl -fsS http://app:8080/readyz >/dev/null 2>&1 && break
	i=$((i + 1))
	sleep 1
done
if ! curl -fsS http://app:8080/readyz; then
	echo "readyz wurde nicht bereit"
	curl -sS -i http://app:8080/readyz || true
	exit 1
fi

echo "== Startseite: Templates sind eingebettet =="
curl -fsS http://app:8080/ | grep -q "Schmetterpause" || {
	echo "Startseite ohne erwarteten Inhalt"
	exit 1
}

echo "== statische Assets: HTMX ist eingebettet =="
size=$(curl -fsS http://app:8080/static/js/htmx.min.js | wc -c)
[ "$size" -gt 1000 ] || {
	echo "htmx.min.js fehlt oder ist leer ($size Bytes)"
	exit 1
}

echo "== HTMX-Fragment: Handler, Repository und Datenbank =="
curl -fsS http://app:8080/fragments/status | grep -q ">erreichbar<" || {
	echo "Statusfragment meldet die Datenbank nicht als erreichbar"
	curl -sS http://app:8080/fragments/status || true
	exit 1
}

echo
echo "verify: alle Pruefungen bestanden"
`
