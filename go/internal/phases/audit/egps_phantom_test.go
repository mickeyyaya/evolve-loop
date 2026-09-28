package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// The verdict shape as acssuite actually writes phantom-bound reds.
const phantomVerdictJSON = `{
  "schema_version": "v11",
  "cycle": 1546,
  "red_count": 2,
  "green_count": 5,
  "verdict": "FAIL",
  "results": [
    {"ac_id": "cycle1546/TestC1546_001_SalvageSnapshotHEADNeverBecomesTheNormalizeBase", "result": "green"},
    {"ac_id": "cycle1544/TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase", "result": "red",
     "phantom_bindings": ["TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor"]},
    {"ac_id": "cycle1544/TestC1544_007_OrdinaryReuseAndUnresolvableAncestorBehaviour", "result": "red",
     "phantom_bindings": ["TestWorktreeReuseBase_OrdinaryCleanReuseBaseIsUnchanged"]}
  ]
}`

func writeVerdictFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "acs-verdict.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestReadACSVerdict_SurfacesPhantomBindings(t *testing.T) {
	redCount, redIDs, phantoms, _, err := readACSVerdict(writeVerdictFile(t, phantomVerdictJSON))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if redCount != 2 || len(redIDs) != 2 {
		t.Fatalf("red accounting must be unchanged; got count=%d ids=%v", redCount, redIDs)
	}
	if len(phantoms) != 2 {
		t.Fatalf("both phantom names must surface; got %v", phantoms)
	}
	for _, want := range []string{
		"TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor",
		"TestWorktreeReuseBase_OrdinaryCleanReuseBaseIsUnchanged",
	} {
		if !contains(phantoms, want) {
			t.Fatalf("phantom %q missing from %v", want, phantoms)
		}
	}
}

func TestEGPSRedMessage_PhantomsCarryTheCure(t *testing.T) {
	msg := egpsRedMessage(2,
		[]string{"cycle1544/TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase"},
		[]string{"TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor"})
	if !strings.Contains(msg, "red_count=2") {
		t.Fatalf("the count survives; got %q", msg)
	}
	if !strings.Contains(msg, "TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor") {
		t.Fatalf("the phantom must be NAMED; got %q", msg)
	}
	if !strings.Contains(msg, "does not resolve") || !strings.Contains(msg, "repoint") {
		t.Fatalf("the message must state the diagnosis and the cure; got %q", msg)
	}
	if !strings.Contains(msg, "do NOT delete") {
		t.Fatalf("the anti-gaming boundary must be stated in the directive itself; got %q", msg)
	}
}

func TestEGPSRedMessage_NoPhantomsIsByteIdentical(t *testing.T) {
	got := egpsRedMessage(1, []string{"cycle1543/TestC1543_002_Whatever"}, nil)
	want := "EGPS: red_count=1 [Whatever] (cycle ships only when red_count==0)"
	if got != want {
		t.Fatalf("phantom-free message must be unchanged:\n got %q\nwant %q", got, want)
	}
}

func TestReadACSVerdict_AllPhantomRedsStillRed(t *testing.T) {
	redCount, _, phantoms, shipEligible, err := readACSVerdict(writeVerdictFile(t, phantomVerdictJSON))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if redCount == 0 {
		t.Fatalf("phantom classification must never zero the red count")
	}
	if shipEligible != nil && *shipEligible {
		t.Fatalf("phantom classification must never confer ship eligibility")
	}
	_ = phantoms
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestRun_PhantomBindingRedEmitsTheCureInTheGateDiagnostic(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte(phantomVerdictJSON), 0o644); err != nil {
		t.Fatalf("stage verdict: %v", err)
	}
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts: fakePromptsFS("body"),
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1546, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("phantom reds are still reds — verdict must be FAIL; got %q", resp.Verdict)
	}
	// The gate DETAIL diagnostic (the egpsRedMessage line) — not the
	// verdict-conflict summary that also mentions EGPS and points here.
	var msg string
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "red_count=2") {
			msg = d.Message
		}
	}
	if msg == "" {
		t.Fatalf("expected the EGPS gate-detail diagnostic; got %+v", resp.Diagnostics)
	}
	if !strings.Contains(msg, "PHANTOM binding") ||
		!strings.Contains(msg, "TestWorktreeReuseBase_SnapshotHeadResolvesToFirstNonSnapshotAncestor") {
		t.Fatalf("the EMITTED diagnostic must name the phantoms: %q", msg)
	}
	if !strings.Contains(msg, "repoint") || !strings.Contains(msg, "do NOT delete") {
		t.Fatalf("the EMITTED diagnostic must carry the cure and the anti-gaming boundary: %q", msg)
	}
}
