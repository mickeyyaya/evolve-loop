package audit

// ciparity_newexport_test.go — RED contract for cycle-1331's
// percycle-audit-apicover-newexport-parity task (scout finding 4). The
// existing two-gate split (apicoverEnforceChangedDefault: touched∩enforced;
// apicoverNewPackageGraduationDefault: new-package blind spot) has never had a
// regression test proving the specific edge: a new EXPORTED symbol landing in
// an EXISTING enforced package via a brand-new file (not a new package, and
// not an edit to an already-tracked file). This is the untested case flagged
// in scout-report.md Finding 4 — the per-cycle gate must catch it exactly as
// CI's whole-repo `apicover -enforce` would, since the new file is recorded
// under the handoff's `files_new` bucket rather than `files_modified`.
//
// changedpkgs.ChangedPackages folds BOTH files_new and files_modified into the
// same changed-package set (changedpkgs.go:85-86), so the hypothesis is that
// no code change is required — this test exists to make that parity a durable,
// provable guard rather than an assumption (scout Hypothesis 2).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestApicoverEnforceChangedDefault_NewExportViaNewFileInExistingPackage_CaughtByGate
// pins the new-export-in-existing-package parity case: `./internal/p` is
// already enforced and already has a clean, exported-symbol-free file
// (x.go). This cycle adds a SECOND file (y.go) to that SAME existing package
// directory, carrying an exported func no test names. The handoff records
// y.go under files_new (a brand-new file, not a modification to x.go) — the
// exact shape scout Finding 4 called out as untested. The per-cycle gate must
// flag it: touched∩enforced scoping must not silently drop a new-file/
// existing-package change the way it (correctly) drops a same-cycle new
// PACKAGE (that's apicoverNewPackageGraduationDefault's job, not this one's).
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

// TestApicoverEnforceChangedDefault_NewExportViaNewFile_NotGraduationGate is the
// negative/boundary half: the SAME fixture must be a no-op for
// apicoverNewPackageGraduationDefault, because ./internal/p is already
// enforced — this scenario belongs to the touched∩enforced gate, not the
// new-package graduation gate. Without this split, a change that quietly
// mis-routes new-file-in-existing-package detection into the graduation gate
// (which explicitly ignores already-enforced packages, ciparity.go:134) would
// go completely unflagged by either gate — the exact silent-drop this task
// closes the proof for.
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
