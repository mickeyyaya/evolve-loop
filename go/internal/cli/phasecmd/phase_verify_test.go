package phasecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func runVerify(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runPhaseVerify(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestPhaseVerify_ValidArtifact_Exit0(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"),
		[]byte("## Changes\n- foo.go\nVerdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runVerify(t, "build", "--workspace="+ws)
	if code != 0 {
		t.Errorf("exit=%d want 0; stderr=%s", code, errb)
	}
}

func TestPhaseVerify_MissingArtifact_NonZeroNamesPath(t *testing.T) {
	ws := t.TempDir()
	code, _, errb := runVerify(t, "build", "--workspace="+ws)
	if code == 0 {
		t.Fatal("exit=0 want non-zero for missing artifact")
	}
	wantPath := filepath.Join(ws, "build-report.md")
	if !strings.Contains(errb, wantPath) {
		t.Errorf("stderr must name the expected path %q; got %q", wantPath, errb)
	}
}

func TestPhaseVerify_JSONOutput(t *testing.T) {
	ws := t.TempDir()
	code, out, _ := runVerify(t, "build", "--workspace="+ws, "--json")
	if code == 0 {
		t.Fatal("want non-zero for missing artifact")
	}
	var res struct {
		OK         bool `json:"ok"`
		Violations []struct {
			Code string `json:"code"`
		} `json:"violations"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("--json must emit valid JSON to stdout: %v\n%s", err, out)
	}
	if res.OK {
		t.Error("ok=true, want false")
	}
}

func TestPhaseVerify_SpaceSeparatedFlags(t *testing.T) {
	ws := t.TempDir()
	code, _, errb := runVerify(t, "build", "--workspace", ws, "--json")
	if code == 0 {
		t.Fatalf("want non-zero for missing artifact; stderr=%s", errb)
	}
	if strings.Contains(errb, "unknown phase") {
		t.Errorf("space-separated flags mis-parsed: %s", errb)
	}
}

func TestPhaseVerify_UnknownPhase_Usage(t *testing.T) {
	code, _, _ := runVerify(t, "nope", "--workspace="+t.TempDir())
	if code != 10 {
		t.Errorf("exit=%d want 10 (usage) for unknown phase", code)
	}
}

func TestPhaseVerify_MissingPhaseArg(t *testing.T) {
	code, _, _ := runVerify(t, "--workspace=/tmp")
	if code != 10 {
		t.Errorf("exit=%d want 10 when phase name omitted", code)
	}
}

func TestPhaseVerify_Advisor_EvolveDirDefault(t *testing.T) {
	// Orchestrator deliverable lives in --evolve-dir.
	ev := t.TempDir()
	if err := os.WriteFile(filepath.Join(ev, "cycle-state.json"),
		[]byte(`{"cycle_id":1,"phase":"tdd"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runVerify(t, "orchestrator", "--evolve-dir="+ev)
	if code != 0 {
		t.Errorf("exit=%d want 0; stderr=%s", code, errb)
	}
}

func TestPhaseVerify_RouterProtocolsUseOwnArtifact(t *testing.T) {
	tests := []struct {
		contract string
		artifact string
		content  string
	}{
		{contract: "router", artifact: "routing-plan.json", content: `[{"phase":"build"}]`},
		{contract: "router-replan", artifact: "routing-replan.json", content: `[{"phase":"audit"}]`},
		{contract: "router-proposal", artifact: "routing-proposal.json", content: `{"next_phase":"audit"}`},
	}

	for _, tt := range tests {
		t.Run(tt.contract, func(t *testing.T) {
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, tt.artifact), []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			code, _, errb := runVerify(t, tt.contract, "--workspace="+ws)
			if code != 0 {
				t.Errorf("exit=%d want 0; stderr=%s", code, errb)
			}
		})
	}
}

func TestPhaseVerify_FailureContextPhaseIO_RespectsStage(t *testing.T) {
	failNoBlock := "## Changes\n- x\n" + phasecontract.RenderVerdictSentinel("build", "FAIL") + "\n"

	t.Run("explicit-off-dormant", func(t *testing.T) {
		t.Setenv("EVOLVE_PHASE_IO", "off") // enforce is the default, so off must be explicit
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(failNoBlock), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, errb := runVerify(t, "build", "--workspace="+ws)
		if code != 0 {
			t.Fatalf("explicit off: failure-context check must be dormant, want exit 0, got %d; stderr=%s", code, errb)
		}
	})

	t.Run("default-enforce-blocks", func(t *testing.T) {
		// No t.Setenv: enforce is the default.
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(failNoBlock), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, errb := runVerify(t, "build", "--workspace="+ws)
		if code != 1 {
			t.Fatalf("default (enforce): want exit 1 (failure_context_missing), got %d; stderr=%s", code, errb)
		}
		if !strings.Contains(errb, "failure") {
			t.Errorf("stderr should name the failure-context correction; got %q", errb)
		}
	})
}

func TestPhaseVerify_StrayInWorktree_Exit1(t *testing.T) {
	ws := t.TempDir()
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, "build-report.md"),
		[]byte("## Changes\n- foo.go\nVerdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runVerify(t, "build", "--workspace="+ws, "--worktree="+wt)
	if code != 1 {
		t.Errorf("exit=%d want 1 (confirmed violation: stray artifact in worktree); stderr=%s", code, errb)
	}
	if !strings.Contains(errb, "stray") && !strings.Contains(errb, "worktree") {
		t.Errorf("stderr should name the stray-in-worktree correction; got %q", errb)
	}
}

func TestPhaseVerify_ExplanationSectionFollowsCycleState(t *testing.T) {
	ws := t.TempDir()
	report := "## Verdict\n**PASS**\n\n## Issues\nnone\n\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	writeState := func(body string) {
		if err := os.WriteFile(filepath.Join(ws, core.RunStateFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeState(`{"cycle_id":7,"phase":"audit","explanation_documentation_version":1}`)
	if code, _, errb := runVerify(t, "audit", "--workspace="+ws); code == 0 || !strings.Contains(errb, "## Explanation Documentation") {
		t.Fatalf("active contract: exit=%d stderr=%q — the self-check must demand the section the gate demands", code, errb)
	}
	writeState(`{"cycle_id":7,"phase":"audit"}`)
	if code, _, errb := runVerify(t, "audit", "--workspace="+ws); code != 0 {
		t.Fatalf("inactive contract must not ask for the section: exit=%d stderr=%q", code, errb)
	}
	if err := os.Remove(filepath.Join(ws, core.RunStateFile)); err != nil {
		t.Fatal(err)
	}
	if code, _, errb := runVerify(t, "audit", "--workspace="+ws); code != 0 || !strings.Contains(errb, core.RunStateFile+" unreadable") {
		t.Fatalf("unreadable state must be reported, never silently treated as inactive: exit=%d stderr=%q", code, errb)
	}
	bws := t.TempDir()
	if err := os.WriteFile(filepath.Join(bws, "build-report.md"), []byte("## Changes\n- foo.go\nVerdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errb := runVerify(t, "build", "--workspace="+bws); code != 0 || strings.Contains(errb, "explanation-documentation") {
		t.Fatalf("build owes no conditional section and must not be warned about one: exit=%d stderr=%q", code, errb)
	}
}
