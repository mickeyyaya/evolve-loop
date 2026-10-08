package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func agedDispatchLog(t *testing.T, root, name string, days int) string {
	t.Helper()
	p := filepath.Join(root, ".evolve", "dispatch-logs", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCmd_PruneEphemeral_DispatchLogsObeyTheGCLogCatalogTTL(t *testing.T) {
	root := t.TempDir()
	gcWritePolicy(t, root, `{"logs":{"dispatch":{"ttl_days":60}}}`)
	kept := agedDispatchLog(t, root, "batch-45d.log", 45)
	pruned := agedDispatchLog(t, root, "batch-70d.log", 70)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	if rc := runPruneEphemeral([]string{"--quiet"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d\n%s", rc, stderr.String())
	}

	if _, err := os.Stat(kept); err != nil {
		t.Errorf("a 45-day log is inside gc.logs.dispatch.ttl_days=60, but it was pruned: %v", err)
	}
	if _, err := os.Stat(pruned); !os.IsNotExist(err) {
		t.Errorf("a 70-day log is past gc.logs.dispatch.ttl_days=60, but it stays: %v", err)
	}
}

func TestCmd_PruneEphemeral_AnExplicitFlagStillOverridesTheCatalog(t *testing.T) {
	root := t.TempDir()
	gcWritePolicy(t, root, `{"logs":{"dispatch":{"ttl_days":60}}}`)
	pruned := agedDispatchLog(t, root, "batch-45d.log", 45)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	if rc := runPruneEphemeral([]string{"--quiet", "--dispatch-log-ttl-days", "40"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d\n%s", rc, stderr.String())
	}

	if _, err := os.Stat(pruned); !os.IsNotExist(err) {
		t.Errorf("--dispatch-log-ttl-days 40 must prune a 45-day log: %v", err)
	}
}
