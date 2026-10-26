package heldproposal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func protectedGuards(path string) bool { return strings.HasPrefix(path, "go/internal/guards/") }

func TestDecide_HoldsAPipelineKindOrAProtectedPathAndNothingElse(t *testing.T) {
	cases := []struct {
		name string
		in   Inputs
		want Decision
	}{
		{"an ordinary lane change", Inputs{Kinds: []string{"bug"}, Changed: []string{"go/internal/bridge/x.go"}, Protected: protectedGuards}, Decision{}},
		{"a pipeline-repair item", Inputs{Kinds: []string{" Pipeline-Repair "}, Protected: protectedGuards}, Decision{Hold: true, Reasons: []string{"kind:pipeline-repair"}}},
		{"a protected path", Inputs{Changed: []string{"go/internal/guards/role.go", "go/internal/guards/role.go"}, Protected: protectedGuards}, Decision{Hold: true, Reasons: []string{"protected:go/internal/guards/role.go"}}},
		{"both causes, sorted", Inputs{Kinds: []string{"pipeline-integrity"}, Changed: []string{"go/internal/guards/a.go"}, Protected: protectedGuards}, Decision{Hold: true, Reasons: []string{"kind:pipeline-integrity", "protected:go/internal/guards/a.go"}}},
		{"no judge holds no path", Inputs{Changed: []string{"go/internal/guards/a.go"}}, Decision{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.in); got.Hold != tc.want.Hold || !reflect.DeepEqual(got.Reasons, tc.want.Reasons) {
				t.Errorf("Decide = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestJudge_OnlyAnApprovalOfTheSameTreeApproves(t *testing.T) {
	approve := Approval{Cycle: 7, Tree: "t1", Decision: DecisionApprove}
	reject := Approval{Cycle: 7, Tree: "t1", Decision: DecisionReject}
	cases := []struct {
		name  string
		a     Approval
		found bool
		tree  string
		want  Verdict
	}{
		{"no approval", Approval{}, false, "t1", Pending},
		{"the approved tree", approve, true, "t1", Approved},
		{"a later tree", approve, true, "t2", Pending},
		{"a rejection of the tree", reject, true, "t1", Rejected},
		{"a rejection of another tree", reject, true, "t2", Pending},
		{"an unknown decision", Approval{Cycle: 7, Tree: "t1", Decision: "maybe"}, true, "t1", Pending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Judge(tc.a, tc.found, tc.tree); got != tc.want {
				t.Errorf("Judge = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatusAfter_NamesTheTerminalStatusOfEachDecision(t *testing.T) {
	if StatusAfter(DecisionApprove) != StatusApproved || StatusAfter(DecisionReject) != StatusRejected {
		t.Errorf("StatusAfter = %q, %q", StatusAfter(DecisionApprove), StatusAfter(DecisionReject))
	}
}

func TestRecordsAndApprovals_RoundTripUnderTheLandingDirectory(t *testing.T) {
	evolveDir := t.TempDir()
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	r := Record{Cycle: 12, Status: StatusHeld, Commit: "c", Tree: "t", Reasons: []string{"kind:pipeline-repair"}, Requires: []string{RequirementOperator, RequirementAdversarialAudit}, At: at}
	if err := WriteRecord(evolveDir, r); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecord(evolveDir, Record{Cycle: 3, Status: StatusWouldHold, Tree: "u", At: at}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(HeldDir(evolveDir), "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, found, err := ReadRecord(evolveDir, 12)
	if err != nil || !found || !reflect.DeepEqual(got, r) {
		t.Errorf("ReadRecord = %+v, %v, %v, want %+v", got, found, err, r)
	}
	all, err := ReadRecords(evolveDir)
	if err != nil || len(all) != 2 || all[0].Cycle != 3 || all[1].Cycle != 12 {
		t.Errorf("ReadRecords = %+v, %v, want cycles 3 and 12 in order, and the other file skipped", all, err)
	}
	a := Approval{Cycle: 12, Tree: "t", Decision: DecisionApprove, Audit: "report.md", At: at}
	if err := WriteApproval(evolveDir, a); err != nil {
		t.Fatal(err)
	}
	gotA, found, err := ReadApproval(evolveDir, 12)
	if err != nil || !found || gotA != a {
		t.Errorf("ReadApproval = %+v, %v, %v, want %+v", gotA, found, err, a)
	}
	if !strings.HasPrefix(ApprovalsDir(evolveDir), filepath.Join(evolveDir, "landing")) || !strings.HasPrefix(HeldDir(evolveDir), filepath.Join(evolveDir, "landing")) {
		t.Error("the records and the approvals must live under the protected .evolve/landing/")
	}
}

func TestReads_AnAbsentFileIsNotFoundAndABrokenOneIsAnError(t *testing.T) {
	evolveDir := t.TempDir()
	if _, found, err := ReadApproval(evolveDir, 1); found || err != nil {
		t.Errorf("absent approval: found %v err %v", found, err)
	}
	if all, err := ReadRecords(evolveDir); all != nil || err != nil {
		t.Errorf("absent held dir: %v, %v", all, err)
	}
	if err := os.MkdirAll(HeldDir(evolveDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(HeldDir(evolveDir), "cycle-4.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecords(evolveDir); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("a broken record must be an error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(ApprovalsDir(evolveDir)), 0o755); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ApprovalsDir(evolveDir), "cycle-5.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadApproval(evolveDir, 5); err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("an unreadable approval must be an error: %v", err)
	}
}

func TestReadRecords_AnUnreadableDirectoryIsAnError(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(evolveDir, "landing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(HeldDir(evolveDir), []byte("a file, not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecords(evolveDir); err == nil || !strings.Contains(err.Error(), "list the hold records") {
		t.Errorf("ReadRecords = %v, want the list error", err)
	}
}

func TestWrites_RefuseAnIncompleteRecordOrApprovalAndReportAFailedWrite(t *testing.T) {
	evolveDir := t.TempDir()
	for _, a := range []Approval{{Tree: "t", Decision: DecisionApprove}, {Cycle: 1, Decision: DecisionApprove}, {Cycle: 1, Tree: "t", Decision: "maybe"}} {
		if err := WriteApproval(evolveDir, a); !errors.Is(err, ErrBadApproval) {
			t.Errorf("WriteApproval(%+v) = %v, want ErrBadApproval", a, err)
		}
	}
	for _, r := range []Record{{Tree: "t", Status: StatusHeld}, {Cycle: 1, Status: StatusHeld}, {Cycle: 1, Tree: "t"}} {
		if err := WriteRecord(evolveDir, r); !errors.Is(err, ErrBadRecord) {
			t.Errorf("WriteRecord(%+v) = %v, want ErrBadRecord", r, err)
		}
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecord(blocked, Record{Cycle: 1, Tree: "t", Status: StatusHeld}); err == nil || !strings.Contains(err.Error(), "create") {
		t.Errorf("WriteRecord under a file = %v, want the create error", err)
	}
	if err := os.MkdirAll(filepath.Join(ApprovalsDir(evolveDir), "cycle-2.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteApproval(evolveDir, Approval{Cycle: 2, Tree: "t", Decision: DecisionReject}); err == nil || !strings.Contains(err.Error(), "write") {
		t.Errorf("WriteApproval over a directory = %v, want the write error", err)
	}
}
