package cycleoutcome

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type dropLane struct {
	root, ws, inbox, sha string
}

func newDropLane(t *testing.T, item map[string]any) dropLane {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/internal/x/x.go", "package x\n")
	repo.Git("add", "go/internal/x/x.go")
	repo.Git("commit", "-q", "-m", "seed")
	l := dropLane{root: repo.Dir, ws: filepath.Join(repo.Dir, ".evolve", "runs", "cycle-7"), inbox: filepath.Join(repo.Dir, ".evolve", "inbox"), sha: repo.Git("rev-parse", "--short", "HEAD")}
	for _, dir := range []string{l.ws, l.inbox} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	item["id"], item["title"], item["kind"] = "item-a", "fixture", "bug"
	body, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(l.inbox, "2026-09-26T00-00-00Z-item-a.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	pin, _ := json.Marshal(map[string]any{"todo_ids": []string{"item-a"}, "goal_hash": "g"})
	if err := os.WriteFile(filepath.Join(l.ws, "lane-scope.json"), pin, 0o644); err != nil {
		t.Fatal(err)
	}
	return l
}

func (l dropLane) drop(t *testing.T, reason string) (map[string]any, []signalcenter.Event) {
	t.Helper()
	decision, _ := json.Marshal(map[string]any{"top_n": []any{}, "dropped": []any{map[string]string{"id": "item-a", "reason": reason}}})
	if err := os.WriteFile(filepath.Join(l.ws, "triage-decision.json"), decision, 0o644); err != nil {
		t.Fatal(err)
	}
	c, events := recordingCenter()
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: l.root, Workspace: l.ws, Cycle: 7, Stderr: io.Discard}.WithSignals(c)); err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	c.Flush()
	return itemRecord(t, l.inbox, "item-a"), *events
}

func hasCode(events []signalcenter.Event, code signalcenter.Code) bool {
	for _, e := range events {
		if e.Code == code {
			return true
		}
	}
	return false
}

func TestApplyNoWork_AStaleDropWhoseEvidenceHoldsIsVerified(t *testing.T) {
	l := newDropLane(t, map[string]any{})

	rec, _ := l.drop(t, "stale: fixed by "+l.sha+", see go/internal/x/x.go:1")

	if rec["evidence_verified"] != true || rec["stale_drops"] != float64(1) {
		t.Fatalf("item = %v, want evidence_verified true and stale_drops 1", rec)
	}
}

func TestApplyNoWork_AStaleDropWhoseEvidenceFailsIsFlagged(t *testing.T) {
	for name, tc := range map[string]struct{ reason, note string }{
		"a sha not in HEAD": {"stale: fixed by 0123456789abcdef0123456789abcdef01234567", "0123456789abcdef0123456789abcdef01234567 is not an ancestor of HEAD"},
		"an absent file":    {"stale: see go/internal/nope/nope.go:12", "go/internal/nope/nope.go is not at HEAD"},
		"no evidence":       {"stale: the code changed", "cites no commit and no file"},
	} {
		t.Run(name, func(t *testing.T) {
			l := newDropLane(t, map[string]any{})

			rec, _ := l.drop(t, tc.reason)

			if rec["evidence_verified"] != false || !strings.Contains(rec["evidence_note"].(string), tc.note) {
				t.Fatalf("item = %v, want evidence_verified false with a note naming %q", rec, tc.note)
			}
		})
	}
}

func TestApplyNoWork_AnAbsentClaimAgainstAScoutReportThatNamesTheIDIsFlagged(t *testing.T) {
	l := newDropLane(t, map[string]any{})
	if err := os.WriteFile(filepath.Join(l.ws, "scout-report.md"), []byte("## Tasks\n- item-a: fix the x\n- item-a again\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, _ := l.drop(t, "assigned id absent from the scout report")

	if rec["evidence_verified"] != false || !strings.Contains(rec["evidence_note"].(string), "scout-report.md names item-a 2 time(s)") {
		t.Fatalf("item = %v, want the unverified negative flagged with the scout count", rec)
	}
	if _, staleCounted := rec["stale_drops"]; staleCounted {
		t.Fatalf("an absent claim is not a stale drop: %v", rec)
	}
}

func TestApplyNoWork_AnOrdinaryDropRecordsNoEvidence(t *testing.T) {
	l := newDropLane(t, map[string]any{})

	rec, _ := l.drop(t, "duplicate of item-b")

	if _, stamped := rec["evidence_verified"]; stamped {
		t.Fatalf("a drop that claims no stale premise and no absence is not judged: %v", rec)
	}
}

func TestApplyNoWork_TheSecondStaleDropIsACodedEscalation(t *testing.T) {
	l := newDropLane(t, map[string]any{"stale_drops": 1, "last_stale_drop_at": "2026-10-01T00:00:00Z"})

	rec, events := l.drop(t, "stale: fixed by "+l.sha)

	if rec["stale_drops"] != float64(2) || !hasCode(events, CodeStaleDropRepeated) {
		t.Fatalf("item = %v, events = %v; want stale_drops 2 and %s", rec, events, CodeStaleDropRepeated)
	}
}

func TestApplyNoWork_APremiseVerifiedAfterTheLastStaleDropRestartsTheCount(t *testing.T) {
	l := newDropLane(t, map[string]any{"stale_drops": 1, "last_stale_drop_at": "2026-10-01T00:00:00Z", "premise_verified_at": "2026-10-02T00:00:00Z"})

	rec, events := l.drop(t, "stale: fixed by "+l.sha)

	if rec["stale_drops"] != float64(1) || hasCode(events, CodeStaleDropRepeated) {
		t.Fatalf("item = %v; a console verification after the last drop restarts the count at 1 with no escalation", rec)
	}
}

func TestVerifyStaleEvidence_AWordOfHexLettersIsNoCommit(t *testing.T) {
	t.Parallel()
	if ev := verifyStaleEvidence(t.TempDir(), "stale: the code was defaced and decaded"); ev.verified || ev.note != "the stale drop cites no commit and no file" {
		t.Fatalf("ev = %+v, want no cited commit", ev)
	}
}

func TestVerifyAbsentClaim_AScoutReportThatDoesNotNameTheIDVerifiesTheClaim(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if ev := verifyAbsentClaim(ws, "item-a"); ev.verified || !strings.Contains(ev.note, "no scout-report.md") {
		t.Fatalf("ev = %+v, want unverifiable without a scout report", ev)
	}
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte("- item-ab: other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := verifyAbsentClaim(ws, "item-a"); !ev.verified {
		t.Fatalf("ev = %+v, want the absent claim verified: item-ab is not item-a", ev)
	}
}

func TestApplyNoWork_AnEvidenceStampThatCannotBeWrittenIsReturned(t *testing.T) {
	l := newDropLane(t, map[string]any{})
	prev := updateItemJSON
	t.Cleanup(func() { updateItemJSON = prev })
	updateItemJSON = func(string, func(map[string]json.RawMessage)) error { return errors.New("disk full") }
	decision, _ := json.Marshal(map[string]any{"top_n": []any{}, "dropped": []any{map[string]string{"id": "item-a", "reason": "stale: fixed by " + l.sha}}})
	if err := os.WriteFile(filepath.Join(l.ws, "triage-decision.json"), decision, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: l.root, Workspace: l.ws, Cycle: 7, Stderr: io.Discard})

	if err == nil || !strings.Contains(err.Error(), "record the drop evidence of item-a: disk full") || len(res.Routed) != 1 {
		t.Fatalf("res = %+v, err = %v; want the route kept and the stamp failure returned", res, err)
	}
}

func TestApplyNoWork_ALaneShapedStaleReasonChecksBothItsFileAndItsCommit(t *testing.T) {
	l := newDropLane(t, map[string]any{})

	rec, _ := l.drop(t, "stale: go/internal/x/x.go:1 (sha "+l.sha+") already does this, and/or.x prose is no file")

	if rec["evidence_verified"] != true || rec["evidence_note"] != "checked 1 commit(s) and 1 file(s) at HEAD" {
		t.Fatalf("item = %v, want the file and the commit checked and a prose token ignored", rec)
	}
}

func TestApplyNoWork_AStaleDropOfAnAlreadyRoutedItemIsStillCounted(t *testing.T) {
	l := newDropLane(t, map[string]any{"route": "console-manual", "stale_drops": 1, "last_stale_drop_at": "2026-10-01T00:00:00Z"})

	rec, events := l.drop(t, "stale: fixed by "+l.sha)

	if rec["stale_drops"] != float64(2) || rec["evidence_verified"] != true || !hasCode(events, CodeStaleDropRepeated) {
		t.Fatalf("item = %v; a console-routed item that a lane stale-drops again is counted and escalated", rec)
	}
}

func TestApplyNoWork_AnInvalidReasonThatClaimsNoAbsenceIsNotJudged(t *testing.T) {
	l := newDropLane(t, map[string]any{})

	rec, _ := l.drop(t, "invalid acceptance: the predicate cannot pass")

	if _, stamped := rec["evidence_verified"]; stamped {
		t.Fatalf("an ordinary invalid reason is no absence claim: %v", rec)
	}
}

func TestVerifyStaleEvidence_ACheckThatRunsOutOfTimeIsUnverifiedNotFailed(t *testing.T) {
	prev := evidenceCheckTimeout
	t.Cleanup(func() { evidenceCheckTimeout = prev })
	evidenceCheckTimeout = 0

	ev := verifyStaleEvidence(t.TempDir(), "stale: fixed by abc1234")

	if ev.verified || ev.note != "the evidence check ran out of time, so the evidence is unverified" {
		t.Fatalf("ev = %+v, want an unverified check, not a failed one", ev)
	}
}
