package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// A cycle whose worktree drifted a SKILL.md (e.g. edited .evolve/profiles/*.json
// without regenerating the phase-facts region) must FAIL audit — the gate that
// would have caught cycle 339's SKILL.md drift before it shipped CI-red on
// TestSkills_NoDrift.
func TestRun_SkillsDrift_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0) // EGPS green, so only the skills gate can FAIL it.
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: body},
		Prompts: fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) {
			return []string{"skills/ship/SKILL.md"}, nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (SKILL.md drift present)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "SKILL.md") {
		t.Errorf("want a diagnostic mentioning SKILL.md drift; got %+v", resp.Diagnostics)
	}
}

func TestRun_SkillsDriftClean_PASSPreserved(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:           &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:          fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) { return nil, nil },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (no skill drift)", resp.Verdict)
	}
}

// Infra error (e.g. the worktree has no phase-registry to load) fails OPEN: warn,
// never brick the cycle on the gate's own inability to run.
func TestRun_SkillsDriftError_FailsOpenWithWarning(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:           &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:          fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) { return nil, errors.New("load phase catalog: no registry") },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (skills infra error fails open)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "skills") {
		t.Errorf("want a warning diagnostic mentioning skills; got %+v", resp.Diagnostics)
	}
}
