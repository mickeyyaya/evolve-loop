package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// A candidate supplied by the auditor must never suppress host execution.
func TestRun_PrestagedGreenCannotSuppressHostRed(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: "## Verdict\n**PASS**\n"},
		Prompts: fakePromptsFS("body"),
		GenerateVerdict: func(req core.PhaseRequest) error {
			writeACSVerdict(t, req.Workspace, 1)
			return nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("auditor's green suppressed the host's failing predicate: verdict=%s", resp.Verdict)
	}
}

func TestRun_FailedCaptureRetiresCandidateAuthority(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	report := "## Verdict\n**PASS**\n<!-- evolve-acs-evidence: {\"version\":1} -->\n"
	phase := New(Config{
		Bridge: &fakeBridge{writeArtifact: report}, Prompts: fakePromptsFS("body"),
		GenerateVerdict:        func(core.PhaseRequest) error { t.Fatal("capture failed but generator ran"); return nil },
		BeginPredicateEvidence: beginPredicateEvidence,
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("want host FAIL, got %s", resp.Verdict)
	}
	body, err := os.ReadFile(filepath.Join(ws, "audit-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "<!-- evolve-acs-evidence:") {
		t.Fatal("failed capture left forged agent receipt eligible for host ledger binding")
	}
	if _, err := os.Stat(filepath.Join(ws, "acs-verdict.json")); !os.IsNotExist(err) {
		t.Fatal("failed capture left agent green at the authoritative verdict path")
	}
}
