package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cycle-1620 audit M1 (the salvage's discovered defect): the rewritten
// handoff parser accepted ANY JSON fence inside "## Handoff to Builder" as the
// declaration, so a complete multi-member declaration that followed an
// unrelated JSON fence (RED-run output, a status object) was shadowed and the
// gate falsely blocked a compliant TDD deliverable. The declaration is the
// first fence that actually DECLARES something.
func TestTDDScopeGate_CompleteDeclarationAfterUnrelatedJSONFenceProceeds(t *testing.T) {
	for _, tc := range []struct{ name, earlier string }{
		{"status object", `{"status":"RED","tests":3}`},
		{"empty declaration shape", `{"slugs":[],"testFiles":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeTriageReport(t, ws, "alpha", "beta")
			body := "## Task: alpha\n## Handoff to Builder\n```json\n" + tc.earlier + "\n```\n```json\n" +
				`{"slugs":["beta","alpha"],"testFiles":["real_test.go"]}` + "\n```\n"
			if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := reviewTDD(t, ws); !got.Approve {
				t.Fatalf("complete declaration shadowed by an earlier unrelated fence: %+v", got)
			}
		})
	}
}

// Positive control for the probe above: the same complete declaration with no
// earlier fence proceeds, and an INCOMPLETE declaration after an unrelated
// fence still blocks — selectivity was restored, not the gate disarmed.
func TestTDDScopeGate_IncompleteDeclarationAfterUnrelatedFenceStillBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha", "beta")
	body := "## Task: alpha\n## Handoff to Builder\n```json\n{\"status\":\"RED\"}\n```\n```json\n" +
		`{"slugs":["alpha"],"testFiles":["real_test.go"]}` + "\n```\n"
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := reviewTDD(t, ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") {
		t.Fatalf("incomplete declaration must still block: %+v", got)
	}
}

// Cycle-1620 audit L1: readTDDScope's ok was never consulted on the
// multi-member path, so an ABSENT test-report.md (the documented fail-open
// ambiguity) blocked a two-member lane as "missing both members".
func TestTDDScopeGate_MissingReportFailsOpenForMultiMemberLane(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha", "beta")
	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("absent test-report.md must fail open (nothing to bind), got %+v", got)
	}
}

// Salvage review CRITICAL 1 controls: the multi-member gate binds to the
// CONTRACT's member set, not the markdown top_n.
//
// Decomposition (the documented norm): the lane is pinned to ONE id while
// triage's markdown top_n lists three working sub-ids; TDD, contracted for the
// pin id, declares it — a single-member lane, never a "multi-member
// commitment missing three sub-ids".
func TestTDDScopeGate_DecompositionSubIDsDoNotBlockThePinnedMember(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "sub-task-a", "sub-task-b", "sub-task-c")
	writeTriageDecision(t, ws, []string{"renamed-work"}, nil)
	if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["renamed-work"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "## Task: renamed-work\n## Handoff to Builder\n```json\n{\"slugs\":[\"renamed-work\"],\"testFiles\":[\"real_test.go\"]}\n```\n"
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("a decomposed single-member lane must not be reconciled against markdown sub-ids: %+v", got)
	}
}

// Deferral: the pin binds {a, b} but the decision defers b, so the contract
// TDD received names only a — declaring exactly {a} proceeds, and declaring
// the deferred member is the unexpected one.
func TestTDDScopeGate_DeferredMemberIsNotCommitted(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha", "beta")
	writeTriageDecision(t, ws, []string{"alpha", "beta"}, []string{"beta"})
	if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["alpha","beta"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "## Task: alpha\n## Handoff to Builder\n```json\n{\"slugs\":[\"alpha\"],\"testFiles\":[\"real_test.go\"]}\n```\n"
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("declaring exactly the non-deferred contract member must proceed: %+v", got)
	}
}
