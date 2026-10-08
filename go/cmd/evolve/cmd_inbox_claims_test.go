package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func claimsRoot(t *testing.T) string {
	t.Helper()
	root := curationRoot(t, map[string]string{
		"processing/cycle-1837/live.json":  `{"id":"live"}`,
		"processing/cycle-1828/stale.json": `{"id":"stale"}`,
		"processing/cycle-1700/.keep":      ``,
	})
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1837")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCmd_InboxClaims_ListsEachClaimWithItsHolderVerdict(t *testing.T) {
	claimsRoot(t)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"claims"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"live  cycle-1837  live", "stale  cycle-1828  stale", "1 empty claim dir(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	stdout.Reset()
	if rc := runInbox([]string{"claims", "--json"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("--json rc = %d stderr = %q", rc, stderr.String())
	}
	var doc inboxmover.ClaimSurvey
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("--json is not a claim survey: %v\n%s", err, stdout.String())
	}
	if len(doc.Claims) != 2 || len(doc.EmptyDirs) != 1 || doc.EmptyDirs[0].Holder.Cycle != 1700 {
		t.Errorf("survey = %+v, want 2 claims and the empty cycle-1700", doc)
	}
	if rc := runInbox([]string{"claims", "--bogus"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("a stray flag: rc = %d, want 10", rc)
	}
}

func TestCmd_InboxRelease_ReleasesAStaleClaimOnceAndRefusesALiveOne(t *testing.T) {
	root := claimsRoot(t)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"release", "stale", "the holder sealed FAIL"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "inbox release: stale released from cycle-1828") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "stale.json")); err != nil {
		t.Errorf("the item must be back at the root: %v", err)
	}
	lines := lifecycleLines(t, root)
	if last := lines[len(lines)-1]; last.Action != "release" || last.TaskID != "stale" || !strings.Contains(last.Message, "the holder sealed FAIL") {
		t.Errorf("ledger tail = %+v, want the release line", last)
	}
	stdout.Reset()
	if rc := runInbox([]string{"release", "stale", "again", "--json"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), `"outcome": "not-claimed"`) {
		t.Errorf("a second release is a no-op success: rc = %d stdout = %q", rc, stdout.String())
	}
	for args, want := range map[string]int{"release live operator": 1, "release never-filed x": 1, "release stale": 10} {
		stderr.Reset()
		if rc := runInbox(strings.Fields(args), nil, &stdout, &stderr); rc != want {
			t.Errorf("%s: rc = %d, want %d (stderr=%q)", args, rc, want, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1837", "live.json")); err != nil {
		t.Errorf("a live claim must stay: %v", err)
	}
}

func TestCmd_InboxReleaseStale_ReleasesEveryStaleClaimAndKeepsTheLiveOne(t *testing.T) {
	root := claimsRoot(t)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"release", "--stale", "wave 82 boundary"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	for _, want := range []string{"inbox release: stale released from cycle-1828 (released): no live lease and no resumable checkpoint", "inbox release: 1 stale claim(s) released"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1837", "live.json")); err != nil {
		t.Errorf("the live claim must stay: %v", err)
	}
	lines := lifecycleLines(t, root)
	if last := lines[len(lines)-1]; last.Action != "release" || last.TaskID != "stale" || !strings.Contains(last.Message, "wave 82 boundary") {
		t.Errorf("ledger tail = %+v, want the release line with the reason", last)
	}
	stdout.Reset()
	if rc := runInbox([]string{"release", "--stale", "again", "--json"}, nil, &stdout, &stderr); rc != 0 || strings.TrimSpace(stdout.String()) != "[]" {
		t.Errorf("a second sweep releases nothing: rc = %d stdout = %q", rc, stdout.String())
	}
	for _, args := range [][]string{{"release", "--stale"}, {"release", "--stale", "a", "b"}, {"release", "--bogus", "a", "b"}} {
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 {
			t.Errorf("%v: rc = %d, want 10", args, rc)
		}
	}
}

func TestCmd_InboxReleaseStale_AConflictExitsOneAndNamesTheClaim(t *testing.T) {
	root := claimsRoot(t)
	seedCurationItems(t, root, map[string]string{"stale.json": `{"id":"other"}`})
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"release", "--stale", "boundary"}, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), "release stale from cycle 1828") {
		t.Errorf("rc = %d stderr = %q; want exit 1 naming the claim that stays", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1828", "stale.json")); err != nil {
		t.Errorf("the conflicting claim must stay: %v", err)
	}
}

func TestCmd_InboxClaimsAndRelease_TextForADuplicateAndAnUnreadableInbox(t *testing.T) {
	root := claimsRoot(t)
	seedCurationItems(t, root, map[string]string{"stale.json": `{"id":"stale"}`, "free.json": `{"id":"free"}`})
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"claims"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "stale  cycle-1828  stale  no live lease and no resumable checkpoint: no cycle state (the inbox root also holds this id)") {
		t.Errorf("rc = %d stdout = %q; want the duplicate note", rc, stdout.String())
	}
	stdout.Reset()
	if rc := runInbox([]string{"release", "stale", "restored by hand"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "inbox release: stale released from cycle-1828; the root copy stays and the duplicate claim copy is removed") {
		t.Errorf("rc = %d stdout = %q; want the duplicate-removed line", rc, stdout.String())
	}
	stdout.Reset()
	if rc := runInbox([]string{"release", "free", "x"}, nil, &stdout, &stderr); rc != 0 || stdout.String() != "inbox release: free is not claimed; nothing to release\n" {
		t.Errorf("rc = %d stdout = %q; want the not-claimed line", rc, stdout.String())
	}
	notADir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(notADir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notADir, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", notADir)
	stderr.Reset()
	if rc := runInbox([]string{"claims"}, nil, &stdout, &stderr); rc != 2 || !strings.HasPrefix(stderr.String(), "inbox claims: list claims: scan the inbox root") {
		t.Errorf("rc = %d stderr = %q; want exit 2 naming the scan", rc, stderr.String())
	}
}

func TestCmd_InboxReleaseStale_TakesTheBoundarysProjectRoot(t *testing.T) {
	root := claimsRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"release", "--stale", "boundary", "--project-root", root}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "inbox release: 1 stale claim(s) released") {
		t.Errorf("rc = %d stdout = %q stderr = %q; want the release under --project-root", rc, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "stale.json")); err != nil {
		t.Errorf("the stale item must be back at the root of --project-root: %v", err)
	}
	if rc := runInbox([]string{"release", "--stale", "boundary", "--project-root"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("--project-root without a value: rc = %d, want 10", rc)
	}
}

func TestCmd_InboxMoverRecoverOrphans_UsesTheOneJudgeAndKeepsALiveClaim(t *testing.T) {
	root := claimsRoot(t)
	var stdout, stderr bytes.Buffer
	if rc := runInboxMover([]string{"recover-orphans"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "[inbox-mover] recover-orphans: released stale from cycle-1828 (released): no live lease and no resumable checkpoint") || !strings.Contains(stderr.String(), "[inbox-mover] recover-orphans: 1 stale claim(s) released") {
		t.Errorf("stderr = %q, want the release lines of the one judge", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1837", "live.json")); err != nil {
		t.Errorf("recover-orphans must keep the live claim: %v", err)
	}
	if last := lifecycleLines(t, root); last[len(last)-1].Action != "release" {
		t.Errorf("ledger tail = %+v, want the one release action", last[len(last)-1])
	}
	seedCurationItems(t, root, map[string]string{"processing/cycle-1820/c.json": `{"id":"c"}`, "c.json": `{"id":"c","weight":1}`})
	stderr.Reset()
	if rc := runInboxMover([]string{"recover-orphans"}, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), "[inbox-mover] ERROR: release c from cycle 1820") {
		t.Errorf("a claim that stays held: rc = %d stderr = %q", rc, stderr.String())
	}
}

func TestCmd_InboxClaims_TakesAProjectRoot(t *testing.T) {
	root := claimsRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"claims", "--project-root", root}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "stale  cycle-1828  stale") {
		t.Errorf("rc = %d stdout = %q; want the claims of --project-root", rc, stdout.String())
	}
	if rc := runInbox([]string{"claims", "--project-root"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("--project-root without a value: rc = %d, want 10", rc)
	}
}

func TestCmd_InboxMoverPromote_RefusesAShortCallAndABadFlag(t *testing.T) {
	claimsRoot(t)
	for args, want := range map[string]string{
		"promote only-id":                        "[inbox-mover] ERROR: usage: promote <task_id> <new_state>",
		"promote stale processed 5 --commit-sha": "[inbox-mover] ERROR: ",
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInboxMover(strings.Fields(args), nil, &stdout, &stderr); rc != 1 || !strings.HasPrefix(stderr.String(), want) {
			t.Errorf("%s: rc = %d stderr = %q; want exit 1 with %q", args, rc, stderr.String(), want)
		}
	}
}
