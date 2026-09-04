package postgres_test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
)

// The guard from issue #163: TruncateAll asks the server what it is about to
// empty and refuses anything that is not named for testing.
//
// Without it `task test:integration` emptied the database the office plays
// on — compose.office.yaml is an overlay on the same `db` service, so `task
// up` and `task office:up` share a volume and a port. The comment on
// TruncateAll gave the separately configured test DSN as the reason that
// could not happen, while the Taskfile pointed that DSN at exactly it.
//
// Aimed at `postgres`, the maintenance database every server has: it exists,
// it holds none of our tables, and its name does not end in _test. The
// assertion is on the wording, because without the guard this call would also
// fail — with "relation does not exist", which is a different thing and would
// let the test pass while protecting nothing.
func TestTruncateAllRefusesADatabaseThatIsNotForTests(t *testing.T) {
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set, skipping integration test", testDSNEnv)
	}

	ctx := t.Context()
	store, err := postgres.Open(ctx, withDatabase(t, dsn, "postgres"))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(store.Close)

	err = postgres.TruncateAll(ctx, store)
	if err == nil {
		t.Fatal("TruncateAll() emptied a database that is not named for tests")
	}
	if !strings.Contains(err.Error(), "refusing to empty") {
		t.Errorf("TruncateAll() = %v, want a refusal naming the database", err)
	}
	if !strings.Contains(err.Error(), `"postgres"`) {
		t.Errorf("the refusal does not say which database it was: %v", err)
	}
}

// The test database itself is accepted, so the guard is a boundary and not a
// blanket refusal that would make every other test in this package pass by
// doing nothing.
func TestTruncateAllAcceptsTheTestDatabase(t *testing.T) {
	store, ctx := newStore(t)

	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("TruncateAll() on the test database: %v", err)
	}
}

// RequireTestDatabase needs no database of its own, so unlike everything else
// in this file it runs in CI as well — which is where a rule about what may
// be emptied is worth having checked.
func TestRequireTestDatabase(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		ok   bool
	}{
		{"the test database", "postgres://u:p@127.0.0.1:5432/schmetterpause_test?sslmode=disable", true},
		{"a second checkout", "postgres://u:p@127.0.0.1:5432/schmetterpause_pr162_test", true},
		{"the office database", "postgres://u:p@127.0.0.1:5432/schmetterpause?sslmode=disable", false},
		{"the maintenance database", "postgres://u:p@127.0.0.1:5432/postgres", false},
		// The keyword/value form reaches the same driver, so it has to reach
		// the same rule. net/url would have read this as nothing at all.
		{"keyword form, test", "host=127.0.0.1 user=u password=p dbname=schmetterpause_test", true},
		{"keyword form, live", "host=127.0.0.1 user=u password=p dbname=schmetterpause", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := postgres.RequireTestDatabase(c.dsn)
			switch {
			case c.ok && err != nil:
				t.Errorf("RequireTestDatabase() = %v, want it accepted", err)
			case !c.ok && err == nil:
				t.Error("RequireTestDatabase() accepted a database that is not for tests")
			case !c.ok && !strings.Contains(err.Error(), "refusing to use"):
				t.Errorf("RequireTestDatabase() = %v, want a refusal", err)
			}
		})
	}
}

// withDatabase swaps the database out of a DSN and leaves everything else —
// host, credentials, sslmode — exactly as the environment set it.
func withDatabase(t *testing.T, dsn, name string) string {
	t.Helper()

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", testDSNEnv, err)
	}
	u.Path = "/" + name
	return u.String()
}
