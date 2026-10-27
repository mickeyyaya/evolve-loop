package auditfix_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const (
	citedTest  = "go/internal/core/retro_remediation_filter_test.go"
	fenceTest  = "go/internal/guards/role_test.go"
	errMessage = "port down"
)

var (
	auditorSeat = auditfix.Seat{Family: "claude", Tier: "deep", Session: "c2035-audit-n1"}
	reviewSeat  = auditfix.Seat{Family: "agy", Tier: "balanced", Session: "c2035-fixreview-n2"}
	errPort     = errors.New(errMessage)
)

type fakePorts struct {
	reviewSeat   auditfix.Seat
	seatErr      error
	snapErr      error
	applyErr     error
	reasons      map[string]string
	patch        auditfix.Patch
	diffErr      error
	review       auditfix.Verdict
	reviewErr    error
	gate         auditfix.Verdict
	gateErr      error
	restoreErr   error
	fixes        []auditfix.FixRequest
	reviews      []auditfix.ReviewRequest
	gateRuns     []string
	restoredFrom []string
}

func passingPorts(lines int, path string) *fakePorts {
	return &fakePorts{
		reviewSeat: reviewSeat,
		reasons:    map[string]string{path: "count only *.json items, as H1 prescribes"},
		patch:      auditfix.Patch{Changes: []auditfix.Change{{Path: path, Lines: lines}}, Diff: "--- a/" + path + "\n+++ b/" + path + "\n"},
		review:     auditfix.Verdict{Pass: true},
		gate:       auditfix.Verdict{Pass: true},
	}
}

func (f *fakePorts) Seat() auditfix.Seat { return auditorSeat }

func (f *fakePorts) ApplyFix(_ context.Context, req auditfix.FixRequest) (map[string]string, error) {
	f.fixes = append(f.fixes, req)
	return f.reasons, f.applyErr
}

func (f *fakePorts) ReviewSeat(_ context.Context, avoid auditfix.Seat) (auditfix.Seat, error) {
	if avoid != auditorSeat {
		return auditfix.Seat{}, errors.New("the review seat was not asked to avoid the auditor seat")
	}
	return f.reviewSeat, f.seatErr
}

func (f *fakePorts) Review(_ context.Context, req auditfix.ReviewRequest) (auditfix.Verdict, error) {
	f.reviews = append(f.reviews, req)
	return f.review, f.reviewErr
}

func (f *fakePorts) Run(_ context.Context, worktree string) (auditfix.Verdict, error) {
	f.gateRuns = append(f.gateRuns, worktree)
	return f.gate, f.gateErr
}

func (f *fakePorts) Snapshot(context.Context, string) (string, error) { return "base-tree", f.snapErr }

func (f *fakePorts) Diff(_ context.Context, _ string, base string) (auditfix.Patch, error) {
	if base != "base-tree" {
		return auditfix.Patch{}, errors.New("the diff did not start at the snapshot")
	}
	return f.patch, f.diffErr
}

func (f *fakePorts) Restore(_ context.Context, _ string, base string) error {
	f.restoredFrom = append(f.restoredFrom, base)
	return f.restoreErr
}

func (f *fakePorts) ports() auditfix.Ports {
	return auditfix.Ports{Auditor: f, Reviewer: f, Gates: f, Worktree: f}
}

func bounds() retryladder.AuditFixBounds {
	return retryladder.AuditFixBounds{MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, MaxTokens: 6000, NoNewProductionLogic: true}
}

func rungInput(t *testing.T, fb phasecontract.FailureBlock) core.RungInput {
	t.Helper()
	return core.RungInput{
		Rung: retryladder.RungAuditFix, Cycle: 2035, Workspace: t.TempDir(), Worktree: "/wt/cycle-2035",
		Choice:  retryladder.AuditFixChoice{Choose: true, Rule: retryladder.AuditFixRuleSmallPrescribed, Input: retryladder.AuditFixInput{Bounds: bounds()}},
		Failure: fb,
	}
}

func smallFix(paths ...string) phasecontract.FailureBlock {
	fb := phasecontract.FailureBlock{Class: "code-audit-fail"}
	for i, p := range paths {
		id := []string{"H1", "M1", "M2"}[i]
		fb.Defects = append(fb.Defects, id+": "+p+":29 inboxFiles counts every entry")
		fb.Prescription = append(fb.Prescription, "Make inboxFiles in "+p+" count only *.json items")
	}
	return fb
}

func enforce(hold bool) auditfix.Config {
	return auditfix.Config{Stage: policy.RetryLadderEnforce, HoldFence: hold}
}

func readRecord(t *testing.T, workspace string) auditfix.Record {
	t.Helper()
	rec, found, err := auditfix.ReadRecord(workspace)
	if err != nil || !found {
		t.Fatalf("ReadRecord = %v, %v, want the %s record", found, err, auditfix.RecordFile)
	}
	return rec
}

func TestHandle_AppliesACitedPatchAndAFreshSeatOfAnotherFamilyReviewsIt(t *testing.T) {
	ports := passingPorts(12, citedTest)
	in := rungInput(t, smallFix(citedTest))
	var handler *auditfix.Handler = auditfix.New(enforce(false), ports.ports())
	var h core.RungHandler = handler

	got, err := h.Handle(context.Background(), in)

	if err != nil || got != (core.RungResult{Handled: true, Reason: auditfix.ReasonPassed}) {
		t.Fatalf("Handle = %+v, %v, want handled and passed", got, err)
	}
	if len(ports.fixes) != 1 || ports.fixes[0].Worktree != in.Worktree || ports.fixes[0].Bounds != bounds() ||
		!slices.Equal(ports.fixes[0].Cited, []string{citedTest}) || ports.fixes[0].Prescriptions[0].Finding != "H1" {
		t.Fatalf("the auditor got %+v, want the cited file, the bounds and the finding H1", ports.fixes)
	}
	if len(ports.reviews) != 1 || ports.reviews[0].Seat != reviewSeat || ports.reviews[0].Seat.Family == auditorSeat.Family ||
		ports.reviews[0].Patch.Diff != ports.patch.Diff || len(ports.reviews[0].Prescriptions) != 1 {
		t.Fatalf("the review got %+v, want the patch and the prescriptions on the other family's seat", ports.reviews)
	}
	if !slices.Equal(ports.gateRuns, []string{in.Worktree}) || len(ports.restoredFrom) != 0 {
		t.Fatalf("gates %v restores %v, want one gate run on the worktree and no restore", ports.gateRuns, ports.restoredFrom)
	}
	rec := readRecord(t, in.Workspace)
	want := auditfix.Prescription{Finding: "H1", Defect: in.Failure.Defects[0], Text: in.Failure.Prescription[0]}
	if rec.Outcome != auditfix.ReasonPassed || rec.Cycle != 2035 || rec.Stage != policy.RetryLadderEnforce ||
		rec.Auditor != auditorSeat || rec.Reviewer != reviewSeat || len(rec.Prescriptions) != 1 || rec.Prescriptions[0] != want ||
		rec.Patch.Changes[0].Reason != ports.reasons[citedTest] || rec.Patch.Diff != ports.patch.Diff {
		t.Fatalf("record = %+v, want the patch, each finding with its prescription and the reason of each change", rec)
	}
}

func TestHandle_TheFixerSeatIsNeverTheReviewerSeat(t *testing.T) {
	for name, seat := range map[string]auditfix.Seat{
		"the auditor session": {Family: "claude", Tier: "deep", Session: auditorSeat.Session},
		"no session":          {Family: "agy", Tier: "balanced"},
	} {
		t.Run(name, func(t *testing.T) {
			ports := passingPorts(12, citedTest)
			ports.reviewSeat = seat
			in := rungInput(t, smallFix(citedTest))

			got, err := auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), in)

			if err != nil || got != (core.RungResult{Reason: auditfix.ReasonNoReviewSeat}) {
				t.Fatalf("Handle = %+v, %v, want a decline for no fresh review seat", got, err)
			}
			if len(ports.fixes)+len(ports.reviews)+len(ports.gateRuns) != 0 {
				t.Fatalf("the auditor or the review ran without a fresh seat: %d fixes, %d reviews", len(ports.fixes), len(ports.reviews))
			}
			if rec := readRecord(t, in.Workspace); rec.Outcome != auditfix.ReasonNoReviewSeat || rec.Reviewer != (auditfix.Seat{}) {
				t.Fatalf("record = %+v, want no_review_seat with no reviewer", rec)
			}
		})
	}
}

func TestHandle_RefusesAPatchOutsideTheCitedFilesOrTheBoundsBeforeTheReview(t *testing.T) {
	for _, tc := range []struct {
		name    string
		changes []auditfix.Change
		refusal string
	}{
		{"an empty patch", nil, auditfix.RefusalEmpty},
		{"a file the prescriptions do not cite", []auditfix.Change{{Path: "go/internal/core/other_test.go", Lines: 2}}, auditfix.RefusalUncited},
		{"too many lines", []auditfix.Change{{Path: citedTest, Lines: 41}}, retryladder.AuditFixRuleLines},
		{"new production logic", []auditfix.Change{{Path: "go/internal/core/floor.go", Lines: 11}}, retryladder.AuditFixRuleCodeLines},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ports := passingPorts(0, citedTest)
			ports.patch.Changes = tc.changes
			in := rungInput(t, smallFix(citedTest, "go/internal/core/floor.go"))

			got, err := auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), in)

			if err != nil || got != (core.RungResult{Handled: true, Escalate: true, Reason: auditfix.ReasonEscalate}) {
				t.Fatalf("Handle = %+v, %v, want one escalation", got, err)
			}
			if len(ports.reviews)+len(ports.gateRuns) != 0 || !slices.Equal(ports.restoredFrom, []string{"base-tree"}) {
				t.Fatalf("reviews %d gates %d restores %v, want no review, no gate and a restore to the snapshot", len(ports.reviews), len(ports.gateRuns), ports.restoredFrom)
			}
			if rec := readRecord(t, in.Workspace); rec.Outcome != auditfix.ReasonEscalate || rec.Refusal != tc.refusal {
				t.Fatalf("record = %+v, want escalate with refusal %q", rec, tc.refusal)
			}
		})
	}
}

func TestHandle_APatchOnAFencePathJoinsTheHeldProposalOnlyAtEnforce(t *testing.T) {
	ports := passingPorts(4, fenceTest)
	in := rungInput(t, smallFix(fenceTest))
	got, err := auditfix.New(enforce(true), ports.ports()).Handle(context.Background(), in)
	if err != nil || got != (core.RungResult{Handled: true, Reason: auditfix.ReasonHeld}) {
		t.Fatalf("Handle = %+v, %v, want the patch held for the operator", got, err)
	}
	if rec := readRecord(t, in.Workspace); !slices.Equal(rec.Held, []string{fenceTest}) || len(ports.reviews) != 1 || len(ports.gateRuns) != 1 {
		t.Fatalf("record %+v reviews %d gates %d, want the fence path held after the review and the gates", rec, len(ports.reviews), len(ports.gateRuns))
	}

	ports = passingPorts(4, fenceTest)
	in = rungInput(t, smallFix(fenceTest))
	got, err = auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), in)
	if err != nil || !got.Escalate || len(ports.reviews) != 0 {
		t.Fatalf("Handle = %+v, %v, reviews %d, want a refused fence patch outside a held proposal", got, err, len(ports.reviews))
	}
	if rec := readRecord(t, in.Workspace); rec.Refusal != auditfix.RefusalFence {
		t.Fatalf("refusal = %q, want %q", rec.Refusal, auditfix.RefusalFence)
	}
}

func TestHandle_AFailedReviewOrARedGateEscalatesOnceThenStops(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edit    func(*fakePorts)
		refusal string
		gates   int
	}{
		{"failed review", func(f *fakePorts) { f.review = auditfix.Verdict{Reasons: []string{"the helper still counts the lock"}} }, auditfix.RefusalReview, 0},
		{"red gate", func(f *fakePorts) { f.gate = auditfix.Verdict{Reasons: []string{"cover-changed 92%"}} }, auditfix.RefusalGate, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ports := passingPorts(12, citedTest)
			tc.edit(ports)
			in := rungInput(t, smallFix(citedTest))
			h := auditfix.New(enforce(false), ports.ports())

			first, err := h.Handle(context.Background(), in)
			if err != nil || first != (core.RungResult{Handled: true, Escalate: true, Reason: auditfix.ReasonEscalate}) {
				t.Fatalf("first Handle = %+v, %v, want one escalation", first, err)
			}
			rec := readRecord(t, in.Workspace)
			if rec.Refusal != tc.refusal || len(rec.Failed) != 1 || len(ports.gateRuns) != tc.gates || len(ports.restoredFrom) != 1 {
				t.Fatalf("record %+v gates %d restores %d, want refusal %q with its reason", rec, len(ports.gateRuns), len(ports.restoredFrom), tc.refusal)
			}

			second, err := h.Handle(context.Background(), in)
			if err != nil || second != (core.RungResult{Handled: true, Stop: true, Reason: auditfix.ReasonExhausted}) {
				t.Fatalf("second Handle = %+v, %v, want a stop with %s", second, err, auditfix.ReasonExhausted)
			}
			if len(ports.fixes) != 1 || readRecord(t, in.Workspace).Outcome != auditfix.ReasonExhausted {
				t.Fatalf("the stop dispatched the auditor again or did not record: %d fixes", len(ports.fixes))
			}
		})
	}
}

func TestHandle_ShadowRecordsWhatItWouldDoAndChangesNoFile(t *testing.T) {
	ports := passingPorts(12, citedTest)
	in := rungInput(t, smallFix(citedTest))

	got, err := auditfix.New(auditfix.Config{Stage: policy.RetryLadderShadow}, ports.ports()).Handle(context.Background(), in)

	if err != nil || got != (core.RungResult{Reason: auditfix.ReasonShadow}) {
		t.Fatalf("Handle = %+v, %v, want an unhandled shadow record", got, err)
	}
	if len(ports.fixes)+len(ports.reviews)+len(ports.gateRuns)+len(ports.restoredFrom) != 0 {
		t.Fatalf("shadow called a port")
	}
	if rec := readRecord(t, in.Workspace); rec.Outcome != auditfix.ReasonShadow || !slices.Equal(rec.Cited, []string{citedTest}) || len(rec.Prescriptions) != 1 {
		t.Fatalf("record = %+v, want the cited files and the prescriptions", rec)
	}
}

func TestHandle_OffOrAnUnchosenRungDoesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		stage  string
		edit   func(*core.RungInput)
		reason string
	}{
		"stage off":     {policy.AuditFixOff, func(*core.RungInput) {}, auditfix.ReasonOff},
		"not chosen":    {policy.RetryLadderEnforce, func(in *core.RungInput) { in.Choice.Choose = false }, auditfix.ReasonNotChosen},
		"another rung":  {policy.RetryLadderEnforce, func(in *core.RungInput) { in.Rung = retryladder.R1 }, auditfix.ReasonNotChosen},
		"an empty rung": {policy.RetryLadderEnforce, func(in *core.RungInput) { in.Rung = "" }, auditfix.ReasonNotChosen},
	} {
		t.Run(name, func(t *testing.T) {
			ports := passingPorts(12, citedTest)
			in := rungInput(t, smallFix(citedTest))
			tc.edit(&in)
			got, err := auditfix.New(auditfix.Config{Stage: tc.stage}, ports.ports()).Handle(context.Background(), in)
			if err != nil || got != (core.RungResult{Reason: tc.reason}) || len(ports.fixes) != 0 {
				t.Fatalf("Handle = %+v, %v, want %s and no port call", got, err, tc.reason)
			}
			if _, found, _ := auditfix.ReadRecord(in.Workspace); found {
				t.Fatalf("%s wrote a record", name)
			}
		})
	}
}

func TestHandle_APortErrorIsLoudAndRestoresTheTree(t *testing.T) {
	for _, tc := range []struct {
		name     string
		edit     func(*fakePorts)
		restores int
	}{
		{"review seat", func(f *fakePorts) { f.seatErr = errPort }, 0},
		{"snapshot", func(f *fakePorts) { f.snapErr = errPort }, 0},
		{"apply", func(f *fakePorts) { f.applyErr = errPort }, 1},
		{"diff", func(f *fakePorts) { f.diffErr = errPort }, 1},
		{"review", func(f *fakePorts) { f.reviewErr = errPort }, 1},
		{"gates", func(f *fakePorts) { f.gateErr = errPort }, 1},
		{"restore after a refusal", func(f *fakePorts) { f.patch.Changes = nil; f.restoreErr = errPort }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ports := passingPorts(12, citedTest)
			tc.edit(ports)
			got, err := auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), rungInput(t, smallFix(citedTest)))
			if !errors.Is(err, errPort) || got.Handled || len(ports.restoredFrom) != tc.restores {
				t.Fatalf("Handle = %+v, %v, restores %d, want the port error and %d restores", got, err, len(ports.restoredFrom), tc.restores)
			}
		})
	}
}

func TestHandle_ARecordThatCannotBeReadOrWrittenIsAnError(t *testing.T) {
	ports := passingPorts(12, citedTest)
	in := rungInput(t, smallFix(citedTest))
	if err := os.WriteFile(filepath.Join(in.Workspace, auditfix.RecordFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), in); err == nil || len(ports.fixes) != 0 {
		t.Fatalf("a corrupt record gave %v and %d fixes, want an error before any fix", err, len(ports.fixes))
	}

	in = rungInput(t, smallFix(citedTest))
	if err := os.Chmod(in.Workspace, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(in.Workspace, 0o755) })
	if _, err := auditfix.New(auditfix.Config{Stage: policy.RetryLadderShadow}, ports.ports()).Handle(context.Background(), in); err == nil {
		t.Fatalf("a record write into a read-only workspace succeeded")
	}
}

type cycle2035Report struct {
	Report  string                     `json:"report"`
	Failure phasecontract.FailureBlock `json:"failure"`
}

func TestReplay_Cycle2035AuditFindingsDecline(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cycle-2035-audit-failures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reports []cycle2035Report
	if err := json.Unmarshal(raw, &reports); err != nil || len(reports) != 4 {
		t.Fatalf("decode %d reports: %v", len(reports), err)
	}
	cfg := policy.Policy{}.WorkflowConfig().RetryLadder.AuditFix
	b := retryladder.AuditFixBounds{MaxFiles: cfg.MaxFiles, MaxLines: cfg.MaxLines, MaxCodeLines: cfg.MaxCodeLines, MaxTokens: cfg.MaxTokens, NoNewProductionLogic: cfg.NoNewProductionLogic}
	for _, r := range reports {
		fb := r.Failure
		choice := retryladder.ChooseAuditFix(retryladder.AuditFixInput{
			Findings: len(fb.Defects), Prescriptions: len(fb.Prescription), AllPrescribed: retryladder.AllPrescribed(fb.Defects, fb.Prescription),
			Estimate: retryladder.EstimateAuditFix(fb.Defects, fb.Prescription), Bounds: b,
		})
		if choice.Choose {
			t.Errorf("%s: ChooseAuditFix chose audit_fix by %q, want a decline", r.Report, choice.Rule)
		}
		ports := passingPorts(12, citedTest)
		in := core.RungInput{Rung: retryladder.RungAuditFix, Cycle: 2035, Workspace: t.TempDir(), Choice: choice, Failure: fb}
		got, err := auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), in)
		if err != nil || got.Handled || len(ports.fixes) != 0 {
			t.Errorf("%s: Handle = %+v, %v, want no fix", r.Report, got, err)
		}
	}
}

func TestHandle_TheRecordPairsEachFindingIDWithItsPrescription(t *testing.T) {
	fb := phasecontract.FailureBlock{
		Defects:      []string{"the helper in " + citedTest + " counts every entry"},
		Prescription: []string{"count only *.json items in " + citedTest, "rerun go test ./internal/core"},
	}
	in := rungInput(t, fb)
	if _, err := auditfix.New(auditfix.Config{Stage: policy.RetryLadderShadow}, passingPorts(1, citedTest).ports()).Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	want := []auditfix.Prescription{
		{Defect: fb.Defects[0], Text: fb.Prescription[0]},
		{Text: fb.Prescription[1]},
	}
	if got := readRecord(t, in.Workspace).Prescriptions; !slices.Equal(got, want) {
		t.Fatalf("prescriptions = %+v, want %+v: a defect without an id prefix has no finding id", got, want)
	}
}

func TestHandle_SumsTheChangesOfOneFileAndKeepsNoIDFromProse(t *testing.T) {
	ports := passingPorts(0, citedTest)
	ports.patch.Changes = []auditfix.Change{{Path: citedTest, Lines: 25}, {Path: citedTest, Lines: 20}}
	in := rungInput(t, phasecontract.FailureBlock{
		Defects:      []string{"Note 1: " + citedTest + ":29 counts every entry"},
		Prescription: []string{"Make inboxFiles in " + citedTest + " count only *.json items"},
	})
	got, err := auditfix.New(enforce(false), ports.ports()).Handle(context.Background(), in)
	if err != nil || !got.Escalate {
		t.Fatalf("Handle = %+v, %v, want an escalation for 45 lines in one file", got, err)
	}
	rec := readRecord(t, in.Workspace)
	if rec.Refusal != retryladder.AuditFixRuleLines || rec.Prescriptions[0].Finding != "" {
		t.Fatalf("refusal %q finding %q, want %q and no finding id from a prefix with a space", rec.Refusal, rec.Prescriptions[0].Finding, retryladder.AuditFixRuleLines)
	}
}
