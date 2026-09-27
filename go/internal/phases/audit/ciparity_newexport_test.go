package audit

// The two-gate split (apicoverEnforceChangedDefault: touched∩enforced;
// apicoverNewPackageGraduationDefault: new-package blind spot) must also catch
// a new exported symbol landing in an existing enforced package via a
// brand-new file (not a new package, and not an edit to an already-tracked
// file), exactly as CI's whole-repo `apicover -enforce` would — since the new
// file is recorded under the handoff's `files_new` bucket rather than
// `files_modified`, and changedpkgs.ChangedPackages folds both buckets into
// the same changed-package set.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestApicoverEnforceChangedDefault_NewExportViaNewFileInExistingPackage_CaughtByGate
// pins the new-export-in-existing-package parity case: `./internal/p` is
// already enforced and already has a clean, exported-symbol-free file (x.go).
// This cycle adds a second file (y.go) to that same existing package
// directory, carrying an exported func no test names, and the handoff records
// y.go under files_new (a brand-new file, not a modification to x.go). The
// per-cycle gate must flag it: touched∩enforced scoping must not silently
// drop a new-file/existing-package change the way it correctly drops a
// same-cycle new package (that's apicoverNewPackageGraduationDefault's job,
// not this one's).
func TestApicoverEnforceChangedDefault_NewExportViaNewFileInExistingPackage_CaughtByGate(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pDir := filepath.Join(goDir, "internal", "p")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing file in the already-enforced package: no exports, clean.
	if err := os.WriteFile(filepath.Join(pDir, "x.go"), []byte(apicoverCleanPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	// NEW file this cycle, same existing package dir, carrying an unnamed
	// export — the parity-gap shape: new export via new file, existing pkg.
	if err := os.WriteFile(filepath.Join(pDir, "y.go"), []byte(apicoverOffenderPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// files_new (not files_modified) — the handoff bucket a genuinely NEW file
	// lands in; proves the gate does not scope only to files_modified.
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/p/y.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	withFakeRunner(t, apicoverPipelineRunner(goDir, nil))
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverEnforceChangedDefault: unexpected error %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("new export via new file in an existing enforced package must be caught (parity with CI's whole-repo apicover -enforce); got no offenders")
	}
}

// TestApicoverEnforceChangedDefault_NewExportViaNewFile_NotGraduationGate is
// the negative/boundary half: the same fixture must be a no-op for
// apicoverNewPackageGraduationDefault, because ./internal/p is already
// enforced — this scenario belongs to the touched∩enforced gate, not the
// new-package graduation gate, which explicitly ignores already-enforced
// packages. Without this split, a change that quietly mis-routes
// new-file-in-existing-package detection into the graduation gate would go
// completely unflagged by either gate.
func TestApicoverEnforceChangedDefault_NewExportViaNewFile_NotGraduationGate(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pDir := filepath.Join(goDir, "internal", "p")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "x.go"), []byte(apicoverCleanPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "y.go"), []byte(apicoverOffenderPkg), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/p/y.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(already-enforced package's new file) = (%v,%v), want (nil,nil) — this shape belongs to apicoverEnforceChangedDefault, not graduation", off, err)
	}
}
