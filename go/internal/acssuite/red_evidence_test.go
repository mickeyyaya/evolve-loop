package acssuite

// red_evidence_test.go — full-output persistence for RED predicates. Three
// batch-18 false-reds (cycles 1173/1175/1178) were undiagnosable after the
// fact because the 600-byte excerpt elides the MIDDLE of the inner go-test
// stream (the same lesson class as 1107/1116/1123, which FailingTests only
// partially fixed: names survive, assertions do not). A red's full stream now
// lands beside the verdict; the bounded retry's outcome is recorded in
// RetryOutcome — NOT Flaky, whose passed-on-retry-only meaning is pinned by
// acs/cycle468 (deterministic reds carry no flaky key and no warnings).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func longRedStream(t *testing.T, needle string) string {
	t.Helper()
	filler := strings.Repeat("x", 1000) // structurally past evidenceMax, not by arithmetic luck
	out := `{"Action":"output","Package":"` + acsPkgBase + `cycle9","Test":"TestC9_002_Bar","Output":"` + filler + needle + filler + `\n"}`
	return goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Ok", "pass"),
		goLine(acsPkgBase+"cycle9", "TestC9_002_Bar", "run"),
		out,
		goLine(acsPkgBase+"cycle9", "TestC9_002_Bar", "fail"),
	)
}

func redOf(t *testing.T, v Verdict) Result {
	t.Helper()
	for _, r := range v.Results {
		if r.ResultStr == "red" {
			return r
		}
	}
	t.Fatal("no red in verdict")
	return Result{}
}

func TestRun_PersistsFullRedEvidenceBeyondExcerptCap(t *testing.T) {
	root, proj := t.TempDir(), t.TempDir()
	const needle = "MID_STREAM_NEEDLE_7731"
	v, err := Run(Options{Root: root, ProjectRoot: proj, Cycle: 9, GoExec: seamGo(longRedStream(t, needle), &fakeExitErr{1})})
	if err != nil {
		t.Fatal(err)
	}
	red := redOf(t, v)
	if strings.Contains(red.EvidenceExcerpt, needle) {
		t.Fatalf("test premise broken: the needle must exceed the excerpt cap to prove the file matters")
	}
	evPath := filepath.Join(proj, ".evolve", "runs", "cycle-9", "acs-red-evidence", "cycle9__TestC9_002_Bar.txt")
	raw, rerr := os.ReadFile(evPath)
	if rerr != nil {
		t.Fatalf("full red evidence must persist beside the verdict (ProjectRoot wins over Root): %v", rerr)
	}
	if !strings.Contains(string(raw), needle) {
		t.Errorf("evidence file must carry the FULL stream incl. the excerpt-elided middle")
	}
	if !strings.HasPrefix(string(raw), "# cycle=9 ac_id=cycle9/TestC9_002_Bar") || !strings.Contains(string(raw), "--- FIRST RUN ---") {
		t.Errorf("evidence must open with the attribution header + first-run label:\n%.200s", raw)
	}
	// Greens never write evidence files.
	entries, _ := os.ReadDir(filepath.Dir(evPath))
	if len(entries) != 1 {
		t.Errorf("exactly the red predicate writes evidence, got %d files", len(entries))
	}
}

func TestRun_RecordsRedOnRetryAndAppendsRetryStream(t *testing.T) {
	root := t.TempDir()
	calls := 0
	seam := func(_ context.Context, _ string, pattern string, _ []string) (string, error) {
		if !strings.HasPrefix(pattern, "./acs/cycle") {
			return "", nil
		}
		calls++
		if calls == 1 {
			return longRedStream(t, "FIRST_RUN_TOKEN"), &fakeExitErr{1}
		}
		return longRedStream(t, "RETRY_RUN_TOKEN"), &fakeExitErr{1}
	}
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seam})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("the bounded retry must have re-run the scope once, got %d calls", calls)
	}
	red := redOf(t, v)
	if red.RetryOutcome != "red-on-retry" {
		t.Errorf("a red that STAYED red on a completed retry must record red-on-retry, got %q", red.RetryOutcome)
	}
	// The cycle-468 pin: a deterministic red carries NO flaky key and adds NO
	// warnings — RetryOutcome and the evidence file are the forensic surface.
	if red.Flaky != "" {
		t.Errorf("flaky must stay passed-on-retry-only (pinned), got %q", red.Flaky)
	}
	if len(v.Warnings) != 0 {
		t.Errorf("a confirmed red must add no warnings (pinned), got %v", v.Warnings)
	}
	evPath := filepath.Join(root, ".evolve", "runs", "cycle-9", "acs-red-evidence", "cycle9__TestC9_002_Bar.txt")
	raw, rerr := os.ReadFile(evPath)
	if rerr != nil {
		t.Fatalf("evidence file: %v", rerr)
	}
	for _, tok := range []string{"FIRST_RUN_TOKEN", "RETRY_RUN_TOKEN", "--- RETRY RUN (still red) ---"} {
		if !strings.Contains(string(raw), tok) {
			t.Errorf("evidence must carry BOTH runs with the retry separator; missing %s", tok)
		}
	}
}

func TestRun_StarvedRetryIsInconclusiveNotConfirmed(t *testing.T) {
	root := t.TempDir()
	calls := 0
	seam := func(_ context.Context, _ string, pattern string, _ []string) (string, error) {
		if !strings.HasPrefix(pattern, "./acs/cycle") {
			return "", nil
		}
		calls++
		if calls == 1 {
			return longRedStream(t, "FIRST_RUN_TOKEN"), &fakeExitErr{1}
		}
		return "", nil // retry produced NOTHING (expired ctx / crash)
	}
	v, err := Run(Options{Root: root, Cycle: 9, GoExec: seam})
	if err != nil {
		t.Fatal(err)
	}
	red := redOf(t, v)
	if red.RetryOutcome != "retry-inconclusive" {
		t.Errorf("a retry that produced no result must NOT read as confirmation, got %q", red.RetryOutcome)
	}
	if red.ResultStr != "red" {
		t.Errorf("inconclusive keeps the first-run red, got %s", red.ResultStr)
	}
}

func TestResultWireShape_FullEvidenceNeverSerializes(t *testing.T) {
	b, err := json.Marshal(Result{ACID: "x/y", ResultStr: "red", fullEvidence: "SECRET_STREAM"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "SECRET_STREAM") {
		t.Fatalf("fullEvidence must stay off the wire (the verdict JSON keeps its excerpt cap): %s", b)
	}
}

func TestWriteRedEvidence_EmptyStreamWritesNothing(t *testing.T) {
	// The synthetic egps/go-lane-parse-error red has no captured stream; it
	// must not mint an empty evidence file.
	root := t.TempDir()
	writeRedEvidence(Options{Root: root, Cycle: 9}, []Result{{ACID: "egps/go-lane-parse-error", ResultStr: "red"}})
	if _, err := os.Stat(filepath.Join(root, ".evolve", "runs", "cycle-9", "acs-red-evidence")); !os.IsNotExist(err) {
		t.Errorf("no stream ⇒ no dir and no file (stat err=%v)", err)
	}
}

func TestWriteRedEvidence_CollidingACIDsGetSuffixedNotOverwritten(t *testing.T) {
	root := t.TempDir()
	rs := []Result{
		{ACID: "cycle84/TestX", ResultStr: "red", fullEvidence: "FROM_CURRENT_SCOPE"},
		{ACID: "cycle84/TestX", ResultStr: "red", fullEvidence: "FROM_REGRESSION_SCOPE"},
	}
	writeRedEvidence(Options{Root: root, Cycle: 9}, rs)
	dir := filepath.Join(root, ".evolve", "runs", "cycle-9", "acs-red-evidence")
	a, _ := os.ReadFile(filepath.Join(dir, "cycle84__TestX.txt"))
	b, berr := os.ReadFile(filepath.Join(dir, "cycle84__TestX-2.txt"))
	if berr != nil || !strings.Contains(string(a), "FROM_CURRENT_SCOPE") || !strings.Contains(string(b), "FROM_REGRESSION_SCOPE") {
		t.Errorf("duplicate ACIDs across scopes must keep BOTH streams (a=%.40s b-err=%v)", a, berr)
	}
}
