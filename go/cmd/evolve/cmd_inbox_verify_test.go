package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func originMainRoot(t *testing.T) (root, sha string) {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	sha = r.Git("rev-parse", "HEAD")
	r.Git("update-ref", "refs/remotes/origin/main", sha)
	seedCurationItems(t, r.Dir, map[string]string{"stale.json": `{"id":"stale","weight":0.4}`, "consumed/gone.json": `{"id":"gone"}`})
	t.Setenv("EVOLVE_PROJECT_ROOT", r.Dir)
	return r.Dir, sha
}

func TestCmd_InboxVerify_StampsTheItemAgainstOriginMainAndRecordsIt(t *testing.T) {
	root, sha := originMainRoot(t)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"verify", "stale", "--evidence", "go/internal/x.go:12 still reads the old field"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	item := itemFields(t, filepath.Join(root, ".evolve", "inbox", "stale.json"))
	if item["premise_verified_sha"] != sha || item["premise_verified_evidence"] != "go/internal/x.go:12 still reads the old field" ||
		item["premise_verified_at"] == nil || item["weight"] != 0.4 {
		t.Errorf("item = %v", item)
	}
	lines := lifecycleLines(t, root)
	if last := lines[len(lines)-1]; last.Action != "verify-premise" || last.TaskID != "stale" || last.GitHead != sha {
		t.Errorf("ledger tail = %+v, want the verify-premise line", last)
	}
}

func TestCmd_InboxVerify_RefusesAnItemNotPendingOrBlankEvidence(t *testing.T) {
	originMainRoot(t)
	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"verify", "gone", "--evidence", "checked"}, 1},
		{[]string{"verify", "never-filed", "--evidence", "checked"}, 1},
		{[]string{"verify", "stale", "--evidence", "  "}, 10},
		{[]string{"verify", "stale"}, 10},
		{[]string{"verify", "--evidence", "checked"}, 10},
		{[]string{"verify", "stale", "extra", "--evidence", "checked"}, 10},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(tc.args, nil, &stdout, &stderr); rc != tc.want {
			t.Errorf("%v: rc = %d, want %d (stderr=%q)", tc.args, rc, tc.want, stderr.String())
		}
	}
}

func TestCmd_InboxVerify_WithoutOriginMainExits2AndStampsNothing(t *testing.T) {
	root := curationRoot(t, map[string]string{"stale.json": `{"id":"stale"}`})
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"verify", "stale", "--evidence", "checked"}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Errorf("rc = %d, want 2 (stderr=%q)", rc, stderr.String())
	}
	if item := itemFields(t, filepath.Join(root, ".evolve", "inbox", "stale.json")); len(item) != 1 {
		t.Errorf("item = %v, want it unstamped", item)
	}
}
