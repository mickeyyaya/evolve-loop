package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestTDDScopeGate_MissingReportFailsOpenForMultiMemberLane(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "alpha", "beta")
	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("absent test-report.md must fail open (nothing to bind), got %+v", got)
	}
}

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
