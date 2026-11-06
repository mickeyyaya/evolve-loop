package releaseconsistency

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSiteContent(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, "landing", "shared")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "content.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSiteVersion(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"current", `{"product":{"version":"v23.2.0"}}`, "OK"},
		{"stale", `{"product":{"version":"v22.27.0"}}`, "MISMATCH"},
		{"no v", `{"product":{"version":"23.2.0"}}`, "MISMATCH"},
		{"another site's content", `{"title":"x"}`, "OK"},
		{"unreadable json", `{`, "NO_MATCH"},
	}
	for _, c := range cases {
		root := t.TempDir()
		writeSiteContent(t, root, c.body)
		if got := checkSiteVersion(root, "23.2.0"); got.Status != c.want {
			t.Errorf("%s: status = %s, want %s (%+v)", c.name, got.Status, c.want, got)
		}
	}
	if got := checkSiteVersion(t.TempDir(), "23.2.0"); got.Status != "OK" {
		t.Errorf("no site: status = %s, want OK", got.Status)
	}
}
