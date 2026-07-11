package audit

// graduation_registration_test.go — cycle-675 AC2 (Task 2,
// build-entry-graduation-guard-audit-regression): the audit-side graduation
// gate (apicoverNewPackageGraduationDefault) landed 2026-07-07 wired via
// NewDefaultWithStageCompact (audit.go:419), but no test binds the PRODUCTION
// constructor to the gate actually firing — the existing coverage exercises
// the default function directly or injects a fake hook via New(Config). This
// inbox item is a 3rd recurrence caused by seams silently disagreeing on
// scope (cycle-652 retro), so the regression bar is: constructed exactly as
// production constructs it, an ungraduated new package FAILs the audit, and
// an enrolled one PASSes. If the CheckApicoverNewPkgGraduation wiring is ever
// dropped from NewDefaultWithStageCompact, the first arm goes green-PASS and
// this test fails.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runDefaultAuditOverNewPkg runs the PRODUCTION-constructed audit phase over a
// fixture whose build handoff introduces go/internal/brandnew, with the given
// .apicover-enforce contents. Subprocess CI gates (vet/acs-durable/apicover)
// are stubbed to exit 0 so the graduation gate — which is in-process — is the
// only gate that can FAIL; the EGPS verdict is pre-staged green.
func runDefaultAuditOverNewPkg(t *testing.T, enforce string) core.PhaseResponse {
	t.Helper()
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte(enforce), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/brandnew/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0) // EGPS green → only a CI-parity gate can FAIL
	withFakeRunner(t, fakeRunFunc(0, "", "", nil))

	phase := NewDefaultWithStageCompact(
		&fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		fakePromptsFS("# Auditor body"), config.StageOff, false)
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: root, Worktree: root, Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return resp
}

// TestNewDefaultWithStageCompact_GraduationGateRegistered: through the
// production constructor, an ungraduated new go/internal package must FAIL the
// audit with a graduation diagnostic (registration + firing), and the SAME
// package enrolled in .apicover-enforce must PASS (the gate is scoped, not a
// blanket FAIL — the anti-no-op arm).
func TestNewDefaultWithStageCompact_GraduationGateRegistered(t *testing.T) {
	resp := runDefaultAuditOverNewPkg(t, "./internal/p\n") // brandnew NOT enrolled
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict = %q, want FAIL — the new-package graduation gate is not registered/firing via NewDefaultWithStageCompact", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, ".apicover-enforce") {
		t.Errorf("want a diagnostic naming the .apicover-enforce graduation obligation; got %+v", resp.Diagnostics)
	}

	resp = runDefaultAuditOverNewPkg(t, "./internal/p\n./internal/brandnew\n") // enrolled
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict = %q, want PASS — an enrolled new package must not trip the graduation gate; diags = %+v", resp.Verdict, resp.Diagnostics)
	}
}
