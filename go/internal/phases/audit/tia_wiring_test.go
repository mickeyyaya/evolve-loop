package audit

// tia_wiring_test.go — the REACHABILITY proof for cycle-1260 Task 1
// (`egps-regression-tia-shadow-wiring`).
//
// A seam whose only caller is a test is dead code. internal/regressiontia can
// be perfectly unit-tested and still never run in production, which is exactly
// the defect this cycle fixes: changedpkgs.ImporterClosure shipped GREEN in
// cycle-1253 with ZERO callers, so the reverse-dependency widening that would
// have caught the cycle-1250 router/routingtest miss never executed once.
//
// These tests therefore drive generateACSVerdict — the real audit-phase
// function that runs the EGPS suite (audit.go:638, calling acssuite.Run at
// :651) — and never call regressiontia directly. The shadow decision must be
// emitted from THAT path or not at all.
//
// Root is a bare temp dir with no go.mod, so acssuite's Go lane is a fast
// no-op (hasGoACSTree false → zero predicates → generateACSVerdict returns
// early without writing a verdict). The TIA emission must happen BEFORE that
// early return: the evidence is about which packages the cycle touched, not
// about whether the suite found predicates.
//
// RED today: internal/regressiontia does not exist and nothing in audit.go
// emits the artifact, so this file fails to COMPILE — a hard non-zero exit,
// never a silent pass.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

// tiaFixture builds a project root carrying a .evolve/policy.json at the given
// regression_tia stage (empty stage ⇒ NO regression_tia block, the checked-in
// production shape) plus the cycle workspace, and returns (root, workspace).
func tiaFixture(t *testing.T, stage string, cycle int) (string, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{}\n"
	if stage != "" {
		body = `{"regression_tia":{"stage":"` + stage + `"}}` + "\n"
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, ws
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestGenerateACSVerdict_ShadowStageEmitsTIADecision is the CRUX wiring proof:
// with regression_tia.stage=shadow, the real audit path emits the decision
// artifact into the cycle workspace.
func TestGenerateACSVerdict_ShadowStageEmitsTIADecision(t *testing.T) {
	root, ws := tiaFixture(t, "shadow", 4242)

	if err := generateACSVerdict(core.PhaseRequest{
		Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: 4242,
	}); err != nil {
		t.Fatalf("generateACSVerdict: %v", err)
	}

	path := filepath.Join(ws, regressiontia.ArtifactName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("shadow stage emitted no %s in the cycle workspace (%v) — the selection logic has no production caller, exactly the dead-code shape ImporterClosure sat in since cycle-1253", regressiontia.ArtifactName, err)
	}
	var d regressiontia.Decision
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	if d.Stage != "shadow" {
		t.Errorf("emitted decision stage = %q, want \"shadow\"", d.Stage)
	}
	if d.WouldSkipCount != len(d.WouldSkip) {
		t.Errorf("would_skip_count = %d but len(would_skip) = %d — the count must project the list", d.WouldSkipCount, len(d.WouldSkip))
	}
}

// TestGenerateACSVerdict_OffStageEmitsNothing is the byte-identical-baseline
// proof and the NEGATIVE axis. The checked-in policy.json has no regression_tia
// block, so this is the LIVE configuration: the audit phase must behave exactly
// as it did before this cycle — no artifact, no computation, no new failure
// mode on the path that grades every cycle.
func TestGenerateACSVerdict_OffStageEmitsNothing(t *testing.T) {
	for _, stage := range []string{"", "off"} {
		root, ws := tiaFixture(t, stage, 4243)

		if err := generateACSVerdict(core.PhaseRequest{
			Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: 4243,
		}); err != nil {
			t.Fatalf("generateACSVerdict (stage=%q): %v", stage, err)
		}

		if _, err := os.Stat(filepath.Join(ws, regressiontia.ArtifactName)); err == nil {
			t.Errorf("stage=%q emitted %s — absent/off policy must leave the audit path untouched", stage, regressiontia.ArtifactName)
		}
	}
}

// TestGenerateACSVerdict_ShadowEmissionNeverFailsTheAudit pins the blast-radius
// bound. Shadow-stage TIA is OBSERVABILITY on the path that grades every cycle:
// a broken/unwritable evidence sink must degrade quietly, never turn a healthy
// audit into an error. Here the workspace does not exist, so Emit cannot write.
func TestGenerateACSVerdict_ShadowEmissionNeverFailsTheAudit(t *testing.T) {
	root, ws := tiaFixture(t, "shadow", 4244)
	missing := filepath.Join(ws, "does", "not", "exist")

	if err := generateACSVerdict(core.PhaseRequest{
		Worktree: root, ProjectRoot: root, Workspace: missing, Cycle: 4244,
	}); err != nil {
		t.Fatalf("generateACSVerdict returned %v — an unwritable shadow-evidence sink must not fail the audit phase; observability may never gate the gate", err)
	}
}
