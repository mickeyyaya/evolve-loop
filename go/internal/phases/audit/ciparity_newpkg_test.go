package audit

// ciparity.go's NewUngraduatedPackages pure function is tested directly in
// internal/ciparity/newpkg_test.go; this file pins the audit-phase gate that
// consumes it. apicoverNewPackageGraduationDefault reads the cycle's changed
// packages plus .apicover-enforce (mirroring apicoverEnforceChangedDefault's
// own resolution), calls ciparity.NewUngraduatedPackages, and returns an
// actionable offender line per ungraduated package when non-empty.
import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestApicoverNewPkgGraduation_OffendersFailAudit mirrors
// TestRun_CIParityGate_Offenders_FAILsAudit's table pattern (audit_ciparity_test.go)
// for the new hook: a gate reporting offenders must FAIL audit even with a
// green EGPS suite and a narrated PASS.
func TestApicoverNewPkgGraduation_OffendersFailAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0) // EGPS green -> only the CI-parity gate can FAIL.
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

// TestApicoverNewPkgGraduationDefault_NoUngraduatedPackages_NoOp: an
// already-graduated changed package (present in .apicover-enforce) must not
// be flagged — the strongest anti-no-op guard against a naive "flag every
// changed internal package" implementation.
func TestApicoverNewPkgGraduationDefault_NoUngraduatedPackages_NoOp(t *testing.T) {
	root, goDir := writeApicoverFixture(t, apicoverCleanPkg) // .apicover-enforce has "./internal/p", handoff touches go/internal/p/x.go
	_ = goDir
	off, err := apicoverNewPackageGraduationDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})
	if err != nil || len(off) != 0 {
		t.Fatalf("apicoverNewPackageGraduationDefault(already-graduated pkg) = (%v,%v), want (nil,nil)", off, err)
	}
}

// TestApicoverNewPkgGraduationDefault_UngraduatedPackageFlagged: a changed
// go/internal/<pkg> absent from .apicover-enforce must be flagged.
func TestApicoverNewPkgGraduationDefault_UngraduatedPackageFlagged(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The graduation gate now inspects the package DIR (test-only packages are
	// vacuous and skipped — ciparity.PackageDirHasProductionGoFiles), so the
	// fixture must materialize the production file its handoff claims.
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

// TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix: the
// audit offender line for an ungraduated package must carry the same
// copy-pasteable prescription the build-entry seam already emits
// (graduationPrescription), not just a terse instruction sentence.
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
	// AC1: the literal .apicover-enforce append line — not a generic
	// instruction — must be present verbatim.
	if !strings.Contains(joined, "append this line to go/.apicover-enforce:  ./internal/brandnew") {
		t.Errorf("offender %q does not carry the literal .apicover-enforce append line — want the same copy-pasteable prescription the build seam emits (phase_bindings_graduation.go:81 graduationPrescription)", joined)
	}
	// AC1: the literal apicover_named_test.go path to create — not a generic
	// instruction — must be present verbatim.
	if !strings.Contains(joined, "go/internal/brandnew/apicover_named_test.go") {
		t.Errorf("offender %q does not carry the literal apicover_named_test.go path — want the same copy-pasteable prescription the build seam emits", joined)
	}
}

// TestApicoverNewPkgGraduationDefault_CmdChangeNotFlagged: a go/cmd/...-only
// change must be a no-op — cmd entrypoints are out of apicover's scope.
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
