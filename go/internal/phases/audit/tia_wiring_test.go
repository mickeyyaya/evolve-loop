package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

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

func TestGenerateACSVerdict_ShadowEmissionNeverFailsTheAudit(t *testing.T) {
	root, ws := tiaFixture(t, "shadow", 4244)
	missing := filepath.Join(ws, "does", "not", "exist")

	if err := generateACSVerdict(core.PhaseRequest{
		Worktree: root, ProjectRoot: root, Workspace: missing, Cycle: 4244,
	}); err != nil {
		t.Fatalf("generateACSVerdict returned %v — an unwritable shadow-evidence sink must not fail the audit phase; observability may never gate the gate", err)
	}
}
