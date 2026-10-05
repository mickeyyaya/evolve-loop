package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const harnessRedVerdictJSON = `{
  "schema_version": "1.0",
  "cycle": 949,
  "red_count": 1,
  "verdict": "FAIL",
  "ship_eligible": false,
  "results": [
    {"ac_id": "egps/go-lane-scope-failed/acs/cycle949", "result": "red", "exit_code": 1,
     "evidence_excerpt": "predicate scope ./acs/cycle949 produced no complete evidence: exit status 1\noutput:\n./p_test.go:3:2: undefined: core.WithJudgeModel"}
  ]
}`

func egpsGateDiagnostic(t *testing.T, resp core.PhaseResponse, marker string) string {
	t.Helper()
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, marker) {
			return d.Message
		}
	}
	t.Fatalf("no diagnostic carries %q; got %+v", marker, resp.Diagnostics)
	return ""
}

func TestRun_AHarnessRedCarriesWhyItsScopeCouldNotRunIntoTheGateDiagnostic(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte(harnessRedVerdictJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	phase := New(Config{Bridge: &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 949, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %q, want FAIL", resp.Verdict)
	}
	msg := egpsGateDiagnostic(t, resp, "red_count=1")
	for _, want := range []string{"egps/go-lane-scope-failed/acs/cycle949", "undefined: core.WithJudgeModel"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the gate diagnostic must carry the real cause %q read from the artifact; got %q", want, msg)
		}
	}
}

func TestRun_ZeroPredicatesFailsWithTheReasonInsteadOfAMissingFile(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(t.TempDir(), "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	phase := New(Config{
		Bridge:          &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: generateACSVerdict,
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 9, ProjectRoot: root, Worktree: root, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %q, want FAIL: a cycle with no predicates cannot pass", resp.Verdict)
	}
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "no such file") {
			t.Errorf("the misleading missing-file diagnostic is back: %q", d.Message)
		}
	}
	msg := egpsGateDiagnostic(t, resp, "egps/no-predicates")
	if !strings.Contains(msg, filepath.Join(root, "go")) {
		t.Errorf("the diagnostic must say why no predicate ran (the module dir without a predicate tree); got %q", msg)
	}
}
