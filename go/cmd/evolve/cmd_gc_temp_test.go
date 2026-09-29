package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunGC_ReapsStalePipelineTempArtifactsPastThePolicyTTL(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	tmp := t.TempDir()
	stale := filepath.Join(tmp, "acs-cycle1515-bin-2718281828")
	foreign := filepath.Join(tmp, "com.apple.launchd.x")
	for _, p := range []string{stale, foreign} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMPDIR", tmp)
	gcWritePolicy(t, projectRoot, `{"temp_ttl_hours":24}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "1 stale pipeline temp artifact(s) would be removed") {
		t.Errorf("--dry-run does not preview the stale artifact:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("--dry-run removed %s", stale)
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale artifact survived `evolve gc`:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("another program's temp entry was removed: %v", err)
	}
}
