package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestApicoverNewPkgGraduation_OffendersFailAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	cfg := Config{
		CheckApicoverNewPkgGraduation: func(core.PhaseRequest) ([]string, error) {
			return []string{"go/internal/brandnew: not in .apicover-enforce"}, nil
		},
		Bridge:  &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts: fakePromptsFS("body"),
	}
	phase := New(cfg)
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (apicover new-package graduation gate reported offenders)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "apicover") {
		t.Errorf("want a diagnostic mentioning apicover; got %+v", resp.Diagnostics)
	}
}

func TestApicoverNewPkgGraduationDefault_NoUngraduatedPackages_NoOp(t *testing.T) {
	root, goDir := writeApicoverFixture(t, apicoverCleanPkg)
	_ = goDir
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(already-graduated pkg) = (%v,%v), want (nil,nil)", off, err)
	}
}

func TestApicoverNewPkgGraduationDefault_UngraduatedPackageFlagged(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "brandnew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "brandnew", "x.go"), []byte("package brandnew\n\n// Exported is real surface.\nfunc Exported() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/brandnew/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverNewPackageGraduationDefault: unexpected error %v", err)
	}
	if len(off) == 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(new ungraduated internal/brandnew) = (%v,nil), want offenders", off)
	}
}

func TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "brandnew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "brandnew", "x.go"), []byte("package brandnew\n\n// Exported is real surface.\nfunc Exported() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/brandnew/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil {
		t.Fatalf("apicoverNewPackageGraduationDefault: unexpected error %v", err)
	}
	if len(off) != 1 {
		t.Fatalf("apicoverNewPackageGraduationDefault(new ungraduated internal/brandnew) = %v, want exactly 1 offender", off)
	}
	joined := strings.Join(off, "\n")
	if !strings.Contains(joined, "append this line to go/.apicover-enforce:  ./internal/brandnew") {
		t.Errorf("offender %q does not carry the literal .apicover-enforce append line — want the same copy-pasteable prescription the build seam emits (phase_bindings_graduation.go:81 graduationPrescription)", joined)
	}
	if !strings.Contains(joined, "go/internal/brandnew/apicover_named_test.go") {
		t.Errorf("offender %q does not carry the literal apicover_named_test.go path — want the same copy-pasteable prescription the build seam emits", joined)
	}
}

func TestApicoverNewPkgGraduationDefault_CmdChangeNotFlagged(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/cmd/evolve/newcmd.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(go/cmd/... only change) = (%v,%v), want (nil,nil) — cmd/ is out of scope", off, err)
	}
}
