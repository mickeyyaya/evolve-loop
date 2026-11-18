package inboxcmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
)

func queuedDeps() Deps {
	d := testDeps()
	d.QueueGate = func(string, string, []string, bool, io.Writer, io.Writer) (int, bool) { return 7, true }
	return d
}

func runWith(d Deps, root string, args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	full := append([]string{args[0], "--project-root", root}, args[1:]...)
	rc := Run(full, strings.NewReader(""), &out, &errb, d)
	return rc, out.String(), errb.String()
}

func TestQueueGate_ReturnsTheQueuedExitCodeFromEveryWriteVerb(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	cases := map[string][]string{
		"edit":          {"edit", "x", "--set", "weight=0.4"},
		"withdraw":      {"withdraw", "x", "reason"},
		"verify":        {"verify", "x", "--evidence", "checked"},
		"route-console": {"route-console", "x", "reason", "3"},
		"route-lane":    {"route-lane", "x", "reason"},
	}
	for name, args := range cases {
		if rc, _, stderr := runWith(queuedDeps(), root, args...); rc != 7 {
			t.Errorf("%s: rc = %d stderr %q, want the queue gate's 7", name, rc, stderr)
		}
	}
}

func TestFinishInboxWrite_JSONGoesThroughPrintVerbJSON(t *testing.T) {
	var out, errb bytes.Buffer
	current.Store(ptr(testDeps()))
	rc := finishInboxWrite(true, inboxWriteResult{Verb: "withdraw", ID: "x"}, "human", &out, &errb)
	if rc != exitOK || !strings.Contains(out.String(), `"x"`) || strings.Contains(out.String(), "human") {
		t.Errorf("rc %d out %q", rc, out.String())
	}
}

func ptr[T any](v T) *T { return &v }

func TestRunInboxLint_UnknownArgIsAUsageError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if rc := runInboxLint(t.TempDir(), []string{"--bogus"}, &out, &errb); rc != exitUsage || !strings.Contains(errb.String(), `unknown arg "--bogus"`) {
		t.Errorf("rc %d stderr %q", rc, errb.String())
	}
}

func TestProtectedBasenameJudge_WarnsWhenTheTrackedTreeIsUnreadable(t *testing.T) {
	t.Parallel()
	var errb bytes.Buffer
	judge := protectedBasenameJudge(t.TempDir(), func(string) bool { return false }, &errb)
	if judge("anything.go") || !strings.Contains(errb.String(), "tracked files unreadable") {
		t.Errorf("stderr %q", errb.String())
	}
}

func TestTakeBlockedBy_ParsesTheFlagAndRejectsAMissingOrBlankValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args    []string
		blocker string
		rest    []string
		ok      bool
	}{
		{[]string{"a", "b"}, "", []string{"a", "b"}, true},
		{[]string{"a", "--blocked-by"}, "", nil, false},
		{[]string{"a", "--blocked-by", "  "}, "", []string{"a"}, false},
		{[]string{"a", "--blocked-by", " t1 ", "b"}, "t1", []string{"a", "b"}, true},
	}
	for _, c := range cases {
		blocker, rest, ok := takeBlockedBy(c.args)
		if blocker != c.blocker || ok != c.ok || strings.Join(rest, ",") != strings.Join(c.rest, ",") {
			t.Errorf("takeBlockedBy(%v) = %q %v %v, want %q %v %v", c.args, blocker, rest, ok, c.blocker, c.rest, c.ok)
		}
	}
}

func TestAckItemFingerprint_ReportsEachUnreadableStageAndIsIdempotent(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if _, _, err := ackItemFingerprint(evolveDir, filepath.Join(evolveDir, "absent.json"), "t"); err == nil {
		t.Error("a missing item must be an error")
	}
	bad := filepath.Join(evolveDir, "bad.json")
	inboxtest.WriteFile(t, bad, "{not json")
	if _, _, err := ackItemFingerprint(evolveDir, bad, "t"); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("invalid json: %v", err)
	}
	item := filepath.Join(evolveDir, "item.json")
	inboxtest.WriteFile(t, item, `{"id":"i","consumed_by":"`+realConsumedByNarrative+`"}`)
	for round := 0; round < 2; round++ {
		fp, found, err := ackItemFingerprint(evolveDir, item, "t")
		if err != nil || !found || fp != incidentFingerprint {
			t.Fatalf("round %d: %q %v %v", round, fp, found, err)
		}
	}
	unreadable := t.TempDir()
	if err := os.Mkdir(filepath.Join(unreadable, "resolved-fingerprints.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ackItemFingerprint(unreadable, item, "t"); err == nil {
		t.Error("a ledger path held by a directory must be an error")
	}
	unwritable := t.TempDir()
	if err := os.Chmod(unwritable, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unwritable, 0o755) })
	if _, _, err := ackItemFingerprint(unwritable, item, "t"); err == nil {
		t.Error("a ledger directory that refuses writes must be an error")
	}
}

func TestReconcileConsumedFingerprints_SkipsNonItemsAndWarnsOnUnreadableOnes(t *testing.T) {
	t.Parallel()
	var errb bytes.Buffer
	ReconcileConsumedFingerprints(filepath.Join(t.TempDir(), "absent"), &errb)
	if errb.Len() != 0 {
		t.Errorf("a missing consumed dir must be silent: %q", errb.String())
	}
	evolveDir := t.TempDir()
	consumed := filepath.Join(evolveDir, "inbox", "consumed")
	inboxtest.WriteFile(t, filepath.Join(consumed, "notes.txt"), "x")
	inboxtest.WriteFile(t, filepath.Join(consumed, "broken.json"), "{")
	if err := os.Mkdir(filepath.Join(consumed, "sub.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	inboxtest.WriteFile(t, filepath.Join(consumed, "ok.json"), `{"id":"ok","consumed_by":"`+realConsumedByNarrative+`"}`)
	ReconcileConsumedFingerprints(evolveDir, &errb)
	if !strings.Contains(errb.String(), "broken.json") || strings.Contains(errb.String(), "notes.txt") || strings.Contains(errb.String(), "sub.json") || strings.Contains(errb.String(), "ok.json") {
		t.Errorf("stderr %q", errb.String())
	}
}

func TestRunInboxList_RefusesAProjectRootWithoutAnEvolveDir(t *testing.T) {
	d := testDeps()
	d.CheckRootHasEvolve = func(string, string, io.Writer) int { return 4 }
	if rc, _, _ := runWith(d, t.TempDir(), "list"); rc != 4 {
		t.Errorf("rc = %d, want the root check's 4", rc)
	}
}

func TestParseInboxCurateArgs_DefaultsTheRootFromTheEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	req, ok := parseInboxCurateArgs(nil, io.Discard)
	if !ok || req.root != root {
		t.Errorf("root %q ok %v, want %q", req.root, ok, root)
	}
}

func TestHost_IsTheZeroDepsBeforeAnyRun(t *testing.T) {
	prev := current.Load()
	current.Store(nil)
	t.Cleanup(func() { current.Store(prev) })
	if d := host(); d.QueueGate != nil || d.PrintVerbJSON != nil {
		t.Errorf("host() = %+v, want the zero Deps", d)
	}
}

func lockedInbox(t *testing.T, body string) string {
	t.Helper()
	root := inboxtest.EmptyRoot(t)
	inboxtest.WriteFile(t, filepath.Join(root, ".evolve", "inbox", "x.json"), body)
	dir := filepath.Join(root, ".evolve", "inbox")
	item := filepath.Join(dir, "x.json")
	for _, p := range []struct {
		path string
		mode os.FileMode
	}{{item, 0o444}, {dir, 0o555}} {
		if err := os.Chmod(p.path, p.mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755); _ = os.Chmod(item, 0o644) })
	return root
}

func TestRouteVerbs_AFailedWriteIsAnIOError(t *testing.T) {
	body := `{"id":"x","kind":"pipeline-repair","files":["go/internal/cycleclassify/classify.go"]}`
	if rc, _, stderr := runWith(testDeps(), lockedInbox(t, body), "route-lane", "x", "operator"); rc != exitIO {
		t.Errorf("route-lane rc %d stderr %q, want %d", rc, stderr, exitIO)
	}
	if rc, _, stderr := runWith(testDeps(), lockedInbox(t, body), "route-console", "x", "operator", "3"); rc != exitIO {
		t.Errorf("route-console rc %d stderr %q, want %d", rc, stderr, exitIO)
	}
}

func TestProtectedBasenameJudge_JudgesTheBasenamesOfTrackedProtectedFiles(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("pkg/guarded.go", "package pkg\n")
	repo.Write("pkg/open.go", "package pkg\n")
	repo.Git("add", ".")
	repo.Git("commit", "-m", "seed")
	var errb bytes.Buffer
	judge := protectedBasenameJudge(repo.Dir, func(rel string) bool { return rel == "pkg/guarded.go" }, &errb)
	if !judge("guarded.go") || judge("open.go") || errb.Len() != 0 {
		t.Errorf("guarded %v open %v stderr %q", judge("guarded.go"), judge("open.go"), errb.String())
	}
}

func TestRouteVerbs_BlockedByNamesAnotherItemOrIsRefused(t *testing.T) {
	t.Parallel()
	seed := func() string {
		root := inboxtest.EmptyRoot(t)
		inboxtest.WriteFile(t, filepath.Join(root, ".evolve", "inbox", "x.json"), `{"id":"x","kind":"bug"}`)
		inboxtest.WriteFile(t, filepath.Join(root, ".evolve", "inbox", "y.json"), `{"id":"y","kind":"bug"}`)
		return root
	}
	if rc, _, stderr := runWith(testDeps(), seed(), "route-console", "x", "op", "3", "--blocked-by", "x"); rc != exitFailed || !strings.Contains(stderr, "names the item itself") {
		t.Errorf("route-console self-block: rc %d stderr %q", rc, stderr)
	}
	lane := seed()
	rc, stdout, stderr := runWith(testDeps(), lane, "route-lane", "x", "op", "--blocked-by", "y", "--json")
	if rc != exitOK || !strings.Contains(stdout, "once y is consumed") {
		t.Errorf("route-lane blocked-by: rc %d stdout %q stderr %q", rc, stdout, stderr)
	}
}
