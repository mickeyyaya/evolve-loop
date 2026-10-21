package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func layoutWorkspace(t *testing.T, cycle string) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func writeHarnessDigestRecord(t *testing.T, root, cycle string, fingerprint string, recurrence int) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "failure-digests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, dir, "cycle-"+cycle+".json", map[string]any{"cycle": 1034, "fingerprint": fingerprint, "recurrence": recurrence})
}

func TestVerifyDisposition_ARetroThatRewritesTheWorkspaceDigestStillFails(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "1034")
	writeHarnessDigestRecord(t, root, "1034", "build|gate-block|harness000001", 2)
	writeMatchingDigest(t, ws)
	writeJSON(t, ws, "disposition.json", validDisposition())

	err := VerifyDisposition(ws)

	if err == nil || !strings.Contains(err.Error(), "harness") || !strings.Contains(err.Error(), "build|gate-block|harness000001") {
		t.Fatalf("VerifyDisposition = %v; want a mismatch against the harness record, though the workspace digest agrees", err)
	}
}

func TestVerifyDisposition_ADispositionThatMatchesTheHarnessRecordPasses(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "1034")
	writeHarnessDigestRecord(t, root, "1034", "audit|gate-block|egps", 0)
	writeJSON(t, ws, "disposition.json", validDisposition())

	if err := VerifyDisposition(ws); err != nil {
		t.Fatalf("VerifyDisposition = %v; want nil when the disposition matches the harness record", err)
	}
}

func TestVerifyDisposition_AMissingHarnessRecordIsUnverified(t *testing.T) {
	t.Parallel()
	_, ws := layoutWorkspace(t, "1034")
	writeMatchingDigest(t, ws)
	writeJSON(t, ws, "disposition.json", validDisposition())

	if err := VerifyDisposition(ws); err == nil || !strings.Contains(err.Error(), "harness digest record") {
		t.Fatalf("VerifyDisposition = %v; want an error, because the workspace copy alone is retro-writable", err)
	}
}

func TestEnsureFailureDigest_WritesTheHarnessRecordOutsideTheWorkspace(t *testing.T) {
	t.Parallel()
	root, ws := layoutWorkspace(t, "77")
	o := &Orchestrator{}

	o.ensureFailureDigest(77, root, ws, "build", "review gate: build handoff floor blocked")

	var inWorkspace, record FailureDigest
	for path, into := range map[string]*FailureDigest{
		filepath.Join(ws, "failure-digest.json"):                           &inWorkspace,
		filepath.Join(root, ".evolve", "failure-digests", "cycle-77.json"): &record,
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatal(err)
		}
	}
	if record.Fingerprint == "" || record.Fingerprint != inWorkspace.Fingerprint || record.Recurrence != inWorkspace.Recurrence || record.Cycle != 77 {
		t.Fatalf("record = %+v, workspace digest = %+v; want the same identity", record, inWorkspace)
	}
}

func TestAssembleFailureDigest_AnUnwritableHarnessRecordIsAnError(t *testing.T) {
	t.Parallel()
	for name, block := range map[string]func(root string){
		"a file at the record dir": func(root string) {
			if err := os.WriteFile(filepath.Join(root, ".evolve", "failure-digests"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"a dir at the record path": func(root string) {
			if err := os.MkdirAll(filepath.Join(root, ".evolve", "failure-digests", "cycle-5.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, ws := layoutWorkspace(t, "5")
			block(root)

			if _, err := AssembleFailureDigest(5, ws, nil); err == nil || !strings.Contains(err.Error(), "harness digest record") {
				t.Fatalf("AssembleFailureDigest = %v; want the record write error", err)
			}
		})
	}
}
