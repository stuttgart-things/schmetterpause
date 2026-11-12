package server_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Opt-in real-browser coverage against the normal handler with disposable users.
func TestPipBrowser(t *testing.T) {
	if os.Getenv("SP_RUN_PIP_BROWSER") != "1" {
		t.Skip("set SP_RUN_PIP_BROWSER=1 and provide Playwright/Chromium")
	}
	h, store, anna, _ := twoBrowsers(t)
	srv := httptest.NewServer(h)
	defer srv.Close()
	id := opponentID(t, store, "Anna")
	cookie, err := json.Marshal(map[string]string{"name": anna.Name, "value": anna.Value})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "node", "../../scripts/pip.browser.cjs")
	cmd.Env = append(os.Environ(), "SP_PREVIEW_BASE_URL="+srv.URL, "SP_PREVIEW_PLAYER_ID="+id, "SP_PREVIEW_COOKIE="+string(cookie))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser: %v\n%s", err, output)
	}
	t.Log(string(output))

	// Run the existing blecki browser regression on the same disposable server.
	cookieFile := filepath.Join(t.TempDir(), "cookies.txt")
	netscape := "#HttpOnly_127.0.0.1\tFALSE\t/\tFALSE\t0\t" + anna.Name + "\t" + anna.Value + "\n"
	if err := os.WriteFile(cookieFile, []byte(netscape), 0600); err != nil {
		t.Fatal(err)
	}
	regression := exec.CommandContext(t.Context(), "node", "../../scripts/blecki_accessories.browser.cjs")
	regression.Env = append(cmd.Env, "SP_PREVIEW_COOKIE_FILE="+cookieFile, "SP_PREVIEW_SCREENSHOT_DIR="+t.TempDir())
	output, err = regression.CombinedOutput()
	if err != nil {
		t.Fatalf("blecki browser regression: %v\n%s", err, output)
	}
	t.Log(string(output))
}
