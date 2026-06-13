package aggregator

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeWorker(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveMode(t *testing.T) {
	t.Parallel()
	tests := map[string]MergeMode{
		"scout":           ModeConcat,
		"research":        ModeConcat,
		"discover":        ModeConcat,
		"audit":           ModeVerdict,
		"learn":           ModeLessons,
		"retrospective":   ModeLessons,
		"retro":           ModeLessons,
		"plan-review":     ModePlanReview,
		"audit-consensus": ModeCrossCLIVote,
		"cross-cli-vote":  ModeCrossCLIVote,
		"unknown":         ModeUnknown,
		"":                ModeUnknown,
	}
	for phase, want := range tests {
		phase, want := phase, want
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			if got := ResolveMode(phase); got != want {
				t.Errorf("ResolveMode(%q) = %v, want %v", phase, got, want)
			}
		})
	}
}

func TestAggregate_RejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var b bytes.Buffer
	rc := Aggregate(Inputs{Phase: "", Output: filepath.Join(dir, "o.md"), Workers: nil}, &b)
	if rc != ExitUsageErr {
		t.Errorf("rc=%d, want %d", rc, ExitUsageErr)
	}
}

func TestAggregate_RejectsNoWorkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var b bytes.Buffer
	rc := Aggregate(Inputs{Phase: "scout", Output: filepath.Join(dir, "o.md")}, &b)
	if rc != ExitUsageErr {
		t.Errorf("rc=%d, want %d", rc, ExitUsageErr)
	}
}

func TestAggregate_MissingWorker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var b bytes.Buffer
	rc := Aggregate(Inputs{
		Phase: "audit", Output: filepath.Join(dir, "o.md"),
		Workers: []string{"/nonexistent.md"},
	}, &b)
	if rc != ExitUsageErr {
		t.Errorf("rc=%d, want %d", rc, ExitUsageErr)
	}
}

func TestAggregate_OutputDirMkdirFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-dir")
	writeWorker(t, filepath.Join(dir, "w.md"), "content")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	rc := Aggregate(Inputs{
		Phase:   "scout",
		Output:  filepath.Join(blocker, "out.md"),
		Workers: []string{filepath.Join(dir, "w.md")},
	}, &b)
	if rc != ExitUsageErr {
		t.Fatalf("rc=%d, want %d", rc, ExitUsageErr)
	}
}

func TestAggregate_MergeReadFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "worker.md")
	writeWorker(t, w, "content")
	calls := 0
	var b bytes.Buffer
	rc := Aggregate(Inputs{
		Phase:   "scout",
		Output:  filepath.Join(dir, "out.md"),
		Workers: []string{w},
		ReadFile: func(path string) ([]byte, error) {
			calls++
			if calls == 1 {
				return os.ReadFile(path)
			}
			return nil, errors.New("read vanished")
		},
	}, &b)
	if rc != ExitUsageErr {
		t.Fatalf("rc=%d, want %d; stderr=%q", rc, ExitUsageErr, b.String())
	}
}

func TestAggregate_OutputRenameFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "worker.md")
	writeWorker(t, w, "content")
	out := filepath.Join(dir, "out.md")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	rc := Aggregate(Inputs{Phase: "scout", Output: out, Workers: []string{w}}, &b)
	if rc != ExitUsageErr {
		t.Fatalf("rc=%d, want %d", rc, ExitUsageErr)
	}
}

func TestAggregate_EmptyWorker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(w, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	rc := Aggregate(Inputs{Phase: "audit", Output: filepath.Join(dir, "o.md"), Workers: []string{w}}, &b)
	if rc != ExitUsageErr {
		t.Errorf("rc=%d, want usage err", rc)
	}
}

func TestAggregate_UnreadableWorker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "worker.md")
	writeWorker(t, w, "content")
	var b bytes.Buffer
	rc := Aggregate(Inputs{
		Phase:    "scout",
		Output:   filepath.Join(dir, "o.md"),
		Workers:  []string{w},
		ReadFile: func(string) ([]byte, error) { return nil, errors.New("permission denied") },
	}, &b)
	if rc != ExitUsageErr {
		t.Fatalf("rc=%d, want %d; stderr=%q", rc, ExitUsageErr, b.String())
	}
	if !strings.Contains(b.String(), "unreadable") {
		t.Fatalf("stderr should explain unreadable worker, got %q", b.String())
	}
}

func TestAggregate_NilReadFileSeamDefaultsToOS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "worker.md")
	writeWorker(t, w, "content")
	out := filepath.Join(dir, "o.md")
	rc := Aggregate(Inputs{
		Phase:   "scout",
		Output:  out,
		Workers: []string{w},
	}, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d, want %d", rc, ExitOK)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(body), "content") {
		t.Fatalf("output missing worker body: %s", body)
	}
}

func TestAggregate_UnknownPhase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "w.md")
	writeWorker(t, w, "content")
	var b bytes.Buffer
	rc := Aggregate(Inputs{Phase: "weird", Output: filepath.Join(dir, "o.md"), Workers: []string{w}}, &b)
	if rc != ExitUsageErr {
		t.Errorf("rc=%d, want usage err", rc)
	}
}

func TestAggregate_ConcatMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "alpha.md")
	w2 := filepath.Join(dir, "beta.md")
	writeWorker(t, w1, "# Scout α\nfindings α")
	writeWorker(t, w2, "# Scout β\nfindings β")
	out := filepath.Join(dir, "out.md")
	var stderr bytes.Buffer
	rc := Aggregate(Inputs{
		Phase: "scout", Output: out, Workers: []string{w1, w2},
		Now: fixedNow(),
	}, &stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	body, _ := os.ReadFile(out)
	s := string(body)
	for _, want := range []string{
		"# Aggregated scout Report",
		"## Worker: alpha",
		"## Worker: beta",
		"findings α",
		"findings β",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestAggregate_VerdictAllPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "v1.md")
	w2 := filepath.Join(dir, "v2.md")
	writeWorker(t, w1, "Verdict: PASS\n\n# Audit 1\nlooks good")
	writeWorker(t, w2, "verdict: pass\n\n# Audit 2\nfine")
	out := filepath.Join(dir, "agg.md")
	rc := Aggregate(Inputs{Phase: "audit", Output: out, Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0", rc)
	}
	body, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(body), "Verdict: PASS") {
		t.Errorf("first line not Verdict: PASS\n%s", body)
	}
}

func TestAggregate_VerdictAnyFail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "v1.md")
	w2 := filepath.Join(dir, "v2.md")
	writeWorker(t, w1, "Verdict: PASS\n")
	writeWorker(t, w2, "Verdict: FAIL\nreason: broken")
	out := filepath.Join(dir, "agg.md")
	rc := Aggregate(Inputs{Phase: "audit", Output: out, Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitVerdictBad {
		t.Errorf("rc=%d, want %d (FAIL)", rc, ExitVerdictBad)
	}
	body, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(body), "Verdict: FAIL") {
		t.Errorf("verdict header wrong: %s", body)
	}
}

func TestAggregate_VerdictMixedToWarn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "v1.md")
	w2 := filepath.Join(dir, "v2.md")
	writeWorker(t, w1, "Verdict: PASS\n")
	writeWorker(t, w2, "Verdict: WARN\nminor\n")
	out := filepath.Join(dir, "agg.md")
	rc := Aggregate(Inputs{Phase: "audit", Output: out, Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0 (WARN ships fluently)", rc)
	}
	body, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(body), "Verdict: WARN") {
		t.Errorf("verdict header wrong: %s", body)
	}
}

func TestAggregate_VerdictMissingFieldIsWarn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := filepath.Join(dir, "v.md")
	writeWorker(t, w, "no verdict line here, just text")
	out := filepath.Join(dir, "agg.md")
	rc := Aggregate(Inputs{Phase: "audit", Output: out, Workers: []string{w}, Now: fixedNow()}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0", rc)
	}
	body, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(body), "Verdict: WARN") {
		t.Errorf("missing verdict treated as WARN; got: %s", body)
	}
}

func TestAggregate_LessonsDedup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "lesson-a.md")
	w2 := filepath.Join(dir, "lesson-b.md")
	writeWorker(t, w1, "preamble\n## Lesson: cache-prefix\nbody-A\n## Lesson: shared\nshared-body-A\n")
	writeWorker(t, w2, "## Lesson: shared\nshared-body-B (duplicate title)\n## Lesson: tdd-failure\nnew-body\n")
	out := filepath.Join(dir, "agg.md")
	rc := Aggregate(Inputs{Phase: "retrospective", Output: out, Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d", rc)
	}
	body, _ := os.ReadFile(out)
	s := string(body)
	if strings.Count(s, "## Lesson: shared") != 1 {
		t.Errorf("dedup failed: expected 1 'shared' lesson, got:\n%s", s)
	}
	if !strings.Contains(s, "## Lesson: cache-prefix") {
		t.Errorf("missing cache-prefix")
	}
	if !strings.Contains(s, "## Lesson: tdd-failure") {
		t.Errorf("missing tdd-failure")
	}
}

func TestAggregate_PlanReviewAllStrong(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mk := func(name string, score float64, verdict string) string {
		p := filepath.Join(dir, name)
		writeWorker(t, p, fmt.Sprintf("Score: %.1f\nVerdict: %s\n", score, verdict))
		return p
	}
	out := filepath.Join(dir, "agg.md")
	rc := Aggregate(Inputs{
		Phase: "plan-review", Output: out,
		Workers: []string{mk("ceo.md", 8, "PROCEED"), mk("eng.md", 7.5, "PROCEED")},
		Now:     fixedNow(),
	}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d", rc)
	}
	body, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(body), "Verdict: PROCEED") {
		t.Errorf("want PROCEED, got:\n%s", body)
	}
}

func TestAggregate_PlanReviewAnyAbort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "a.md")
	w2 := filepath.Join(dir, "b.md")
	writeWorker(t, w1, "Score: 9\nVerdict: PROCEED\n")
	writeWorker(t, w2, "Score: 8\nVerdict: ABORT\nsecurity concern\n")
	rc := Aggregate(Inputs{
		Phase: "plan-review", Output: filepath.Join(dir, "o.md"),
		Workers: []string{w1, w2}, Now: fixedNow(),
	}, os.Stderr)
	if rc != ExitVerdictBad {
		t.Errorf("rc=%d, want 1 (ABORT)", rc)
	}
}

func TestAggregate_PlanReviewAvgUnder5IsAbort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "a.md")
	w2 := filepath.Join(dir, "b.md")
	writeWorker(t, w1, "Score: 3\nVerdict: REVISE\n")
	writeWorker(t, w2, "Score: 4\nVerdict: REVISE\n")
	rc := Aggregate(Inputs{Phase: "plan-review", Output: filepath.Join(dir, "o.md"), Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitVerdictBad {
		t.Errorf("rc=%d, want 1 (ABORT)", rc)
	}
}

func TestAggregate_PlanReviewWeakLensReturnsRevise(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "a.md")
	w2 := filepath.Join(dir, "b.md")
	writeWorker(t, w1, "Score: 9\nVerdict: PROCEED\n")
	writeWorker(t, w2, "Score: 4\nVerdict: REVISE\n")
	// avg = 6.5 (>=5) but weak lens — should REVISE
	rc := Aggregate(Inputs{Phase: "plan-review", Output: filepath.Join(dir, "o.md"), Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d", rc)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "o.md"))
	if !strings.HasPrefix(string(body), "Verdict: REVISE") {
		t.Errorf("want REVISE, got:\n%s", body)
	}
}

func TestAggregate_CrossCLIPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "claude.md")
	w2 := filepath.Join(dir, "gemini.md")
	w3 := filepath.Join(dir, "codex.md")
	writeWorker(t, w1, "Verdict: PASS\n")
	writeWorker(t, w2, "Verdict: PASS\n")
	writeWorker(t, w3, "Verdict: WARN\n")
	rc := Aggregate(Inputs{
		Phase: "cross-cli-vote", Output: filepath.Join(dir, "o.md"),
		Workers: []string{w1, w2, w3}, Now: fixedNow(),
	}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0 (2/3 PASS, quorum=2)", rc)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "o.md"))
	if !strings.Contains(string(body), "MAJORITY-PASS with FAIL-VETO") {
		t.Errorf("missing protocol text: %s", body)
	}
}

func TestAggregate_CrossCLIVetoOnAnyFail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "claude.md")
	w2 := filepath.Join(dir, "gemini.md")
	writeWorker(t, w1, "Verdict: PASS\n")
	writeWorker(t, w2, "Verdict: FAIL\nblocker\n")
	rc := Aggregate(Inputs{Phase: "audit-consensus", Output: filepath.Join(dir, "o.md"), Workers: []string{w1, w2}, Now: fixedNow()}, os.Stderr)
	if rc != ExitVerdictBad {
		t.Errorf("rc=%d, want 1 (FAIL veto)", rc)
	}
}

func TestAggregate_CrossCLIBelowQuorumIsWarn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w1 := filepath.Join(dir, "claude.md")
	w2 := filepath.Join(dir, "gemini.md")
	w3 := filepath.Join(dir, "codex.md")
	writeWorker(t, w1, "Verdict: PASS\n")
	writeWorker(t, w2, "Verdict: WARN\n")
	writeWorker(t, w3, "Verdict: WARN\n")
	// 1 PASS, no FAIL, quorum=2 — should WARN
	rc := Aggregate(Inputs{Phase: "cross-cli-vote", Output: filepath.Join(dir, "o.md"), Workers: []string{w1, w2, w3}, Now: fixedNow()}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0 (WARN ships)", rc)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "o.md"))
	if !strings.HasPrefix(string(body), "Verdict: WARN") {
		t.Errorf("want WARN, got:\n%s", body)
	}
}

func TestExtractVerdict_MultipleFormsRecognized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []struct {
		body string
		want string
	}{
		{"Verdict: PASS\n", "PASS"},
		{"verdict: fail\n", "FAIL"},
		{"  Verdict:   warn  \n", "WARN"},
		{"# heading\n\nVerdict: PASS with comment\n", "PASS"},
		{"no verdict here", ""},
	}
	for i, tc := range tests {
		p := filepath.Join(dir, fmt.Sprintf("w-%d.md", i))
		writeWorker(t, p, tc.body)
		got := extractVerdict(p)
		if got != tc.want {
			t.Errorf("body %q: got %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestExtractScore_VariousFormats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []struct {
		body string
		want float64
	}{
		{"Score: 8\n", 8},
		{"score: 7.5\n", 7.5},
		{"  Score:  3.14 anything\n", 3.14},
		{"no score", 0},
	}
	for i, tc := range tests {
		p := filepath.Join(dir, fmt.Sprintf("s-%d.md", i))
		writeWorker(t, p, tc.body)
		got := extractScore(p)
		if got != tc.want {
			t.Errorf("body %q: got %v, want %v", tc.body, got, tc.want)
		}
	}
}

func fixedNow() func() time.Time {
	return func() time.Time {
		return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	}
}

// avoid import of fmt for tests
func init() {
	// no-op — keep package clean
}
