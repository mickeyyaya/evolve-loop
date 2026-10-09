package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestApicoverEnforceChangedDefault_NewExportViaNewFileInExistingPackage_CaughtByGate(t *testing.T) {
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

	withFakeRunner(t, apicoverPipelineRunner(goDir, nil))
	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverEnforceChangedDefault: unexpected error %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("new export via new file in an existing enforced package must be caught (parity with CI's whole-repo apicover -enforce); got no offenders")
	}
}

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
