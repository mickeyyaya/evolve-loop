package cycleoutcome

// refusal_route_test.go — the FAIL closeout's per-item breaker. A phase-refusal
// (cycleclassify.ClassPhaseRefusal, read from the C1 record) is TASK-level, so
// the drain's failure_count bump and the ADR-0072 S5 ceiling finally reach the
// item; and the one refusal that is deterministic AND operator-owned — a top_n
// card naming a protected surface — routes the committed item to
// console-manual on the FIRST hit, in place, before the drain releases it
// (docs/incidents/2026-09-14-triage-refusal-poison-loop.md: one item drew nine
// lanes, cycles 1650–1675, because none of this was reachable).

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func writeRefusalRecord(t *testing.T, ws, code, subject string) {
	t.Helper()
	entries := []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS"},
		{Phase: "triage", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{
			Severity: cyclestate.SeverityError, Code: code, Subject: subject,
			Message: `top_n card "poison" names protected surface "go/internal/bridge/x.go" — control-plane changes go through the console route`,
		}}},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(phasetiming.Path(ws), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemRecord(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(dir, id)
	if err != nil {
		t.Fatalf("%s not in %s: %v", id, dir, err)
	}
	body, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func recordingCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	events := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *events = append(*events, e) })
	return c, events
}

func TestIsTaskLevelResult_RefusalsBlameByCode(t *testing.T) {
	if IsTaskLevelFailure(cycleclassify.ClassPhaseRefusal) {
		t.Error("the class alone cannot say whose fault a refusal is")
	}
	for code, want := range map[string]bool{cyclestate.DiagCodeTriageProtectedSurface: true, cyclestate.DiagCodeTriageTopNEmpty: true, cyclestate.DiagCodeTriageCommitmentInvalid: false, "": false} {
		if got := IsTaskLevelResult(cycleclassify.Result{Class: cycleclassify.ClassPhaseRefusal, Marker: code}); got != want {
			t.Errorf("IsTaskLevelResult(%q) = %v, want %v", code, got, want)
		}
	}
	if !IsTaskLevelResult(cycleclassify.Result{Class: cycleclassify.ClassBuildFail}) {
		t.Error("the class rule still holds for the other classes")
	}
}

// The lane's triage claimed the item into processing/cycle-7/ and then refused
// it for a protected surface: the closeout routes it console-manual IN PLACE,
// the drain releases it to the root already routed, no failure_count is
// bumped, the ceiling does not quarantine it (the route is the disposition),
// and the stream carries INBOX_ITEM_ROUTED_CONSOLE naming the refusal.
func TestApplyFailure_ProtectedSurfaceRefusal_RoutesTheItemConsole(t *testing.T) {
	root, inbox, ws := seedProject(t, "poison", []string{"poison"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "poison", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeTriageProtectedSurface, "poison")
	c, events := recordingCenter()
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard).WithSignals(c)
	in.Ceiling = 1
	if in.SystemLevel {
		t.Fatal("a coded refusal is task-level")
	}
	if in.Refusal != cyclestate.DiagCodeTriageProtectedSurface || in.RefusalSubject != "poison" || !strings.Contains(in.RefusalDetail, "protected surface") {
		t.Fatalf("FailureInputsFor must carry the refusal code, subject and detail: %+v", in)
	}
	if _, err := ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if !hasItem(t, inbox, "poison") {
		t.Fatal("the routed item returns to the inbox root (not quarantine/)")
	}
	if _, err := os.Stat(filepath.Join(inbox, "quarantine")); err == nil {
		t.Error("the route is the disposition — no quarantine park")
	}
	rec := itemRecord(t, inbox, "poison")
	if rec["route"] != "console-manual" || rec["routed_cycle"] != float64(7) || !strings.Contains(rec["routed_reason"].(string), "names protected surface") {
		t.Errorf("item = %v", rec)
	}
	if _, bumped := rec["failure_count"]; bumped {
		t.Errorf("no failure_count on a routed item: %v", rec)
	}
	var routed, claimNotFound bool
	for _, e := range *events {
		switch e.Code {
		case "INBOX_ITEM_ROUTED_CONSOLE":
			routed = e.Cycle == 7 && e.Fields["task_id"] == "poison" && strings.Contains(e.Fields["reason"], "protected surface")
		case "INBOX_CLAIM_NOT_FOUND":
			claimNotFound = true
		}
	}
	if !routed {
		t.Errorf("INBOX_ITEM_ROUTED_CONSOLE with the cycle, id and reason: %+v", *events)
	}
	if claimNotFound {
		t.Errorf("an already-claimed committed id is not a not-found claim: %+v", *events)
	}
	// The breaker closes: the next lane's claim is refused.
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "poison", "8"); err == nil {
		t.Error("a routed item must refuse the next lane claim")
	}
}

// A refusal that is NOT operator-owned (an empty top_n) is task-level but not
// routed: the bump reaches the item and the S5 ceiling parks it.
func TestApplyFailure_OtherRefusal_BumpsAndQuarantinesAtCeiling(t *testing.T) {
	root, inbox, ws := seedProject(t, "flaky", []string{"flaky"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "flaky", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeTriageTopNEmpty, "")
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	in.Ceiling = 1
	if in.SystemLevel || in.Refusal != cyclestate.DiagCodeTriageTopNEmpty {
		t.Fatalf("inputs = %+v", in)
	}
	res, err := ApplyFailure(in)
	if err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if hasItem(t, inbox, "flaky") || len(res.Quarantined) != 1 {
		t.Errorf("at ceiling 1 the coded refusal parks the item: root=%v res=%+v", hasItem(t, inbox, "flaky"), res)
	}
	if rec := itemRecord(t, filepath.Join(inbox, "quarantine"), "flaky"); rec["route"] != nil || rec["failure_count"] != float64(1) {
		t.Errorf("bumped, not routed: %v", rec)
	}
}

// An uncoded FAIL keeps the pre-fix reading: system-level, nothing bumped.
func TestFailureInputsFor_UncodedFailStaysSystemLevel(t *testing.T) {
	root, _, ws := seedProject(t, "x", []string{"x"})
	writeRefusalRecord(t, ws, "", "")
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	if !in.SystemLevel || in.Refusal != "" {
		t.Errorf("inputs = %+v", in)
	}
}

// Two committed ids, one refused: only the Subject is routed; the other was
// never worked (triage stopped the cycle) and returns untouched — no bump.
func TestApplyFailure_TwoCommitted_OnlyTheSubjectIsRouted(t *testing.T) {
	root, inbox, ws := seedProject(t, "poison", []string{"poison", "innocent"})
	body, _ := json.Marshal(map[string]any{"id": "innocent", "title": "fixture innocent", "kind": "bug"})
	if err := os.WriteFile(filepath.Join(inbox, "2026-07-29T00-00-01Z-innocent.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"poison", "innocent"} {
		if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, id, "7"); err != nil {
			t.Fatalf("claim %s: %v", id, err)
		}
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeTriageProtectedSurface, "poison")
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	in.Ceiling = 1
	if _, err := ApplyFailure(in); err != nil {
		t.Fatal(err)
	}
	if in.SystemLevel {
		t.Error("the caller's inputs are never rewritten")
	}
	if rec := itemRecord(t, inbox, "poison"); rec["route"] != "console-manual" {
		t.Errorf("poison = %v", rec)
	}
	if rec := itemRecord(t, inbox, "innocent"); rec["route"] != nil || rec["failure_count"] != nil {
		t.Errorf("innocent must be released untouched: %v", rec)
	}
}

// A protected-surface refusal whose Subject is empty or names an id outside
// the committed set (an LLM mis-copy) routes NOTHING: the fallback is the
// task-level bump, and stderr says why (architecture review HIGH-1).
func TestApplyFailure_SubjectNotCommitted_FallsBackToTheBump(t *testing.T) {
	for _, subject := range []string{"", "someone-elses-item"} {
		root, inbox, ws := seedProject(t, "flaky", []string{"flaky"})
		if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "flaky", "7"); err != nil {
			t.Fatal(err)
		}
		writeRefusalRecord(t, ws, cyclestate.DiagCodeTriageProtectedSurface, subject)
		var stderr strings.Builder
		in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, &stderr)
		in.Ceiling = 1
		res, err := ApplyFailure(in)
		if err != nil {
			t.Fatal(err)
		}
		if hasItem(t, inbox, "flaky") || len(res.Quarantined) != 1 {
			t.Errorf("subject %q: the bump reaches the item instead: root=%v res=%+v", subject, hasItem(t, inbox, "flaky"), res)
		}
		if subject != "" && !strings.Contains(stderr.String(), "not in the committed set") {
			t.Errorf("subject %q: the fallback says why: %q", subject, stderr.String())
		}
	}
}

// TRIAGE_COMMITMENT_INVALID is stamped on I/O faults reading the decision —
// the pipeline's fault: system-level, nothing bumped (architecture review HIGH-2).
func TestFailureInputsFor_CommitmentInvalidIsSystemLevel(t *testing.T) {
	root, inbox, ws := seedProject(t, "x", []string{"x"})
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "x", "7"); err != nil {
		t.Fatal(err)
	}
	writeRefusalRecord(t, ws, cyclestate.DiagCodeTriageCommitmentInvalid, "")
	in := FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	if !in.SystemLevel || in.Refusal != cyclestate.DiagCodeTriageCommitmentInvalid {
		t.Fatalf("inputs = %+v", in)
	}
	in.Ceiling = 1
	if _, err := ApplyFailure(in); err != nil {
		t.Fatal(err)
	}
	if rec := itemRecord(t, inbox, "x"); rec["failure_count"] != nil {
		t.Errorf("an I/O fault charges nobody: %v", rec)
	}
}
