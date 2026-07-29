package cycleoutcome

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
)

// seedProject writes an inbox item plus the cycle workspace's
// triage-decision.json and returns (projectRoot, inboxDir, workspace).
func seedProject(t *testing.T, id string, committed []string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"id": id, "title": "fixture " + id, "kind": "bug"})
	if err := os.WriteFile(filepath.Join(inbox, "2026-07-29T00-00-00Z-"+id+".json"), body, 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	rows := make([]map[string]string, 0, len(committed))
	for _, c := range committed {
		rows = append(rows, map[string]string{"id": c})
	}
	dec, _ := json.Marshal(map[string]any{"top_n": rows})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), dec, 0o644); err != nil {
		t.Fatalf("write decision: %v", err)
	}
	return root, inbox, ws
}

func hasItem(t *testing.T, dir, id string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, rErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rErr != nil {
			continue
		}
		var doc struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(b, &doc) == nil && doc.ID == id {
			return true
		}
	}
	return false
}

// TestApplyFailureQuarantinesAtCeiling proves the seam reaches the durable
// lifecycle: at Ceiling 1 the committed id is parked in quarantine/ and gone
// from the inbox root. This is the transition the wave path could not reach at
// all before the seam existed.
func TestApplyFailureQuarantinesAtCeiling(t *testing.T) {
	root, inbox, ws := seedProject(t, "poison", []string{"poison"})

	res, err := ApplyFailure(FailureInputs{
		ProjectRoot: root,
		Workspace:   ws,
		Cycle:       7,
		Ceiling:     1,
		Reason:      "cycle-failure-release",
		Stderr:      io.Discard,
	})
	if err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if hasItem(t, inbox, "poison") {
		t.Errorf("'poison' still at the inbox root; at Ceiling 1 it must be quarantined")
	}
	if !hasItem(t, filepath.Join(inbox, "quarantine"), "poison") {
		t.Errorf("'poison' is not in inbox/quarantine/; the S5 ceiling did not take effect")
	}
	if len(res.Quarantined) == 0 {
		t.Errorf("OutcomeResult.Quarantined is empty; the caller cannot log what it parked")
	}
}

// TestApplyFailureSystemLevelDoesNotQuarantine pins ADR-0072 AC4: a pipeline
// failure releases the committed id but must never walk it toward the ceiling.
func TestApplyFailureSystemLevelDoesNotQuarantine(t *testing.T) {
	root, inbox, ws := seedProject(t, "healthy", []string{"healthy"})

	if _, err := ApplyFailure(FailureInputs{
		ProjectRoot: root,
		Workspace:   ws,
		Cycle:       7,
		Ceiling:     1,
		SystemLevel: true,
		Stderr:      io.Discard,
	}); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if !hasItem(t, inbox, "healthy") {
		t.Errorf("'healthy' left the inbox root on a SYSTEM-level failure; S3 releases, never quarantines")
	}
	if hasItem(t, filepath.Join(inbox, "quarantine"), "healthy") {
		t.Errorf("'healthy' was quarantined by a SYSTEM-level failure; a quota/infra storm is not the task's fault")
	}
}

// TestCommittedIDsForReadsTopN covers the workspace reader, including the
// absent-decision case that must yield nil rather than an error.
func TestCommittedIDsForReadsTopN(t *testing.T) {
	_, _, ws := seedProject(t, "a", []string{"a", "b"})

	got := CommittedIDsFor(ws)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("CommittedIDsFor = %v; want [a b]", got)
	}
	if missing := CommittedIDsFor(t.TempDir()); missing != nil {
		t.Errorf("CommittedIDsFor(no decision) = %v; want nil", missing)
	}
}

// TestIsTaskLevelFailure pins the blame rule both call sites depend on.
func TestIsTaskLevelFailure(t *testing.T) {
	for _, tc := range []struct {
		c    cycleclassify.Classification
		want bool
	}{
		{cycleclassify.ClassBuildFail, true},
		{cycleclassify.ClassAuditFail, true},
		{cycleclassify.ClassShipGateConfig, true},
		{cycleclassify.ClassInfrastructure, false},
		{cycleclassify.ClassIntegrityBreach, false},
	} {
		if got := IsTaskLevelFailure(tc.c); got != tc.want {
			t.Errorf("IsTaskLevelFailure(%v) = %v; want %v", tc.c, got, tc.want)
		}
	}
}

// TestFailureInputsForDerivesCeilingAndBlame proves the derivation helper wires
// a real ceiling and flips SystemLevel off for a task-level classification —
// without it each call site would re-derive (and could re-derive differently).
func TestFailureInputsForDerivesCeilingAndBlame(t *testing.T) {
	root, _, ws := seedProject(t, "a", []string{"a"})
	evolveDir := filepath.Join(root, ".evolve")

	in := FailureInputsFor(root, evolveDir, ws, 7, io.Discard)

	if in.Cycle != 7 || in.ProjectRoot != root || in.Workspace != ws {
		t.Errorf("FailureInputsFor did not carry identity through: %+v", in)
	}
	if in.Ceiling <= 0 {
		t.Errorf("FailureInputsFor Ceiling = %d; want the policy default (>0) or quarantine is disabled everywhere", in.Ceiling)
	}
	if in.Reason == "" {
		t.Errorf("FailureInputsFor Reason is empty; the ledger loses why the item moved")
	}
	if want := !IsTaskLevelFailure(cycleclassify.Classify(ws).Class); in.SystemLevel != want {
		t.Errorf("FailureInputsFor SystemLevel = %v; want %v (must agree with the shared blame rule)", in.SystemLevel, want)
	}
}
