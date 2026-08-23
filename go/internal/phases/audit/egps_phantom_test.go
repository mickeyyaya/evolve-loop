package audit

// egps_phantom_test.go — the EGPS gate-block must carry the phantom-binding
// cure when the classification exists.
//
// acssuite now names bound tests that never ran (Result.PhantomBindings). If
// that classification dies inside acs-verdict.json while the gate still emits
// a bare "EGPS: red_count=N", nothing changed for the operator — the exact
// producer-with-no-consumer shape this week keeps re-finding. These pin the
// consumer half: readACSVerdict surfaces the names, egpsRedMessage renders the
// cure, and the classification NEVER changes the verdict (anti-gaming: a
// phantom red is still red).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// The REAL cycle-1546 shape, as the new producer writes it.
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

// The message must name the phantoms AND the cure — a bare count is exactly
// what cost the 1539-1546 streak a console forensic session.
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

// NO-REGRESSION: without phantoms the message is byte-identical to before.
func TestEGPSRedMessage_NoPhantomsIsByteIdentical(t *testing.T) {
	got := egpsRedMessage(1, []string{"cycle1543/TestC1543_002_Whatever"}, nil)
	want := "EGPS: red_count=1 [Whatever] (cycle ships only when red_count==0)"
	if got != want {
		t.Fatalf("phantom-free message must be unchanged:\n got %q\nwant %q", got, want)
	}
}

// Anti-gaming: a verdict whose reds are ALL phantoms still blocks. The cure is
// a correction, never a skip.
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

// THE END-TO-END WIRING — through phase.Run, the path production uses. Every
// test above builds inputs by hand, so all of them pass even if the gate call
// site passes an empty phantom list (the mutation that survived them: seventh
// NOT-WIRED of the week). A pre-staged phantom verdict goes in; the emitted
// FAIL diagnostic must come out carrying the names and the cure.
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
