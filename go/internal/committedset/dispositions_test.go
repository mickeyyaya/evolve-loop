package committedset

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDispositions_OnlyAnsweringBucketsAnswer pins which of triage's
// non-commitment buckets ANSWER for an id (F30, cycle 1682): a lane scoped to
// one item whose triage dropped it with a reason did its job, and must end as
// planned no-work rather than a claim failure. Deferral is deliberately NOT an
// answer — cycle 1623's triage narrated a claim failure as a deferral
// (empty_commitment_claimable_test.go), so a deferred id stays claimable work.
func TestDispositions_OnlyAnsweringBucketsAnswer(t *testing.T) {
	ws := t.TempDir()
	write(t, ws, DecisionFile, map[string]any{
		"top_n":    ids("committed"),
		"deferred": ids("deferred-only"),
		"dropped": []map[string]string{
			{"id": "dropped-shipped", "reason": "already-shipped-cycle-1679"},
			{"id": "dropped-silent", "reason": "  "},
		},
		"skip_shipped":   []map[string]string{{"task_id": "shipped", "git_sha": "abc123"}, {"task_id": "shipped-no-sha"}},
		"skip_rejected":  []map[string]string{{"task_id": "rejected"}},
		"escalate_block": []map[string]any{{"task_id": "escalated", "reason": "console-routed"}, {"task_id": "escalated-count", "fail_count": 2}, {"task_id": "escalated-bare"}},
	})

	got := Dispositions(ws)
	want := []Disposition{
		{ID: "escalated", Bucket: "escalate_block", Reason: "console-routed"},
		{ID: "escalated-count", Bucket: "escalate_block", Reason: "fail_count 2"},
		{ID: "escalated-bare", Bucket: "escalate_block", Reason: "escalated without a stated reason"},
		{ID: "rejected", Bucket: "skip_rejected", Reason: "triage reported an earlier rejection (inbox/rejected)"},
		{ID: "shipped", Bucket: "skip_shipped", Reason: "git_sha abc123"},
		{ID: "dropped-shipped", Bucket: "dropped", Reason: "already-shipped-cycle-1679"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Dispositions =\n %+v\nwant\n %+v\n(a silent drop, a sha-less skip, a deferral and a commitment answer for nothing)", got, want)
	}
}

// TestDispositions_TheMostSevereBucketWinsAndNothingIsFabricated: an id named
// in two buckets is reported once, by its most severe bucket — a drop's
// "already-shipped" reason never masks a fraudulent-commit escalation (F30
// architecture review m1) — and an absent or malformed decision answers for
// nothing: this package never fabricates.
func TestDispositions_TheMostSevereBucketWinsAndNothingIsFabricated(t *testing.T) {
	ws := t.TempDir()
	write(t, ws, DecisionFile, map[string]any{
		"dropped":        []map[string]string{{"id": "twice", "reason": "already-shipped"}},
		"escalate_block": []map[string]string{{"task_id": "twice", "reason": "fraudulent-commit:def456"}},
	})
	if got := Dispositions(ws); len(got) != 1 || got[0] != (Disposition{ID: "twice", Bucket: "escalate_block", Reason: "fraudulent-commit:def456"}) {
		t.Fatalf("an id in two buckets is one disposition, the most severe bucket's: %+v", got)
	}

	if got := Dispositions(t.TempDir()); got != nil {
		t.Fatalf("an absent decision answers for nothing: %+v", got)
	}
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, DecisionFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Dispositions(bad); got != nil {
		t.Fatalf("a malformed decision answers for nothing: %+v", got)
	}
}

// TestDispositionsFrom_ReadsTheDecisionBody is the body-level reader other
// packages project from (inboxmover already holds decision bodies): the same
// answers as the workspace reader, and a malformed body answers for nothing.
func TestDispositionsFrom_ReadsTheDecisionBody(t *testing.T) {
	got := DispositionsFrom([]byte(`{"dropped":[{"id":"x","reason":"superseded"}],"deferred":[{"id":"y"}]}`))
	if len(got) != 1 || got[0] != (Disposition{ID: "x", Bucket: "dropped", Reason: "superseded"}) {
		t.Fatalf("DispositionsFrom = %+v", got)
	}
	if got := DispositionsFrom([]byte("{not json")); got != nil {
		t.Fatalf("a malformed body answers for nothing: %+v", got)
	}
}
