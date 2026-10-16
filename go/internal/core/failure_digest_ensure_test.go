package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureFailureDigest_WritesDigestWithoutLedger(t *testing.T) {
	t.Parallel()

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	ws := t.TempDir()
	root := t.TempDir()
	o.ensureFailureDigest(1046, root, ws, "", "")
	raw, err := os.ReadFile(filepath.Join(ws, "failure-digest.json"))
	if err != nil {
		t.Fatalf("digest must be written even with no ledger and no audit-fail-reason: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("digest must be non-empty")
	}
}

func TestEnsureFailureDigest_Idempotent(t *testing.T) {
	t.Parallel()

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	ws := t.TempDir()
	root := t.TempDir()
	o.ensureFailureDigest(7, root, ws, "", "")
	first, _ := os.ReadFile(filepath.Join(ws, "failure-digest.json"))
	o.ensureFailureDigest(7, root, ws, "", "")
	second, _ := os.ReadFile(filepath.Join(ws, "failure-digest.json"))
	if string(first) != string(second) {
		t.Fatalf("digest must be deterministic/idempotent:\n%s\nvs\n%s", first, second)
	}
}

func TestEnsureFailureDigest_MissingWorkspaceNeverWritesToWorkingDirectory(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.ensureFailureDigest(7, t.TempDir(), "", "audit", "resumed audit failed")
	for _, name := range []string{"audit-fail-reason.json", "failure-digest.json"} {
		if _, err := os.Stat(filepath.Join(cwd, name)); !os.IsNotExist(err) {
			t.Errorf("missing workspace wrote unrelated working-directory artifact %s: %v", name, err)
		}
	}
}
