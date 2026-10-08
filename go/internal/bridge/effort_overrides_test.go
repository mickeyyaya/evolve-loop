package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProfile_RefusesTheRetiredEffortFields(t *testing.T) {
	for _, body := range []string{
		`{"name":"builder","effort_level":"medium"}`,
		`{"name":"builder","effort_overrides":{"deep":"high"}}`,
	} {
		path := filepath.Join(t.TempDir(), "builder.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadProfile(path)
		if err == nil || !strings.Contains(err.Error(), "evolve cli-routing migrate") {
			t.Errorf("LoadProfile(%s) err = %v, want a refusal that names evolve cli-routing migrate", body, err)
		}
	}
}
