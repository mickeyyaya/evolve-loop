package research

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// filekb_recall_test.go — the policy-resolved recall bound (cycle-1494,
// `sleep-time-kb-consolidation`).

// recallCorpus writes n all-matching lessons with descending confidence, so the
// deterministic ranking has a stable, non-tied order a prefix assertion can
// rely on.
func recallCorpus(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		body := fmt.Sprintf(`- id: recall-%02d
  pattern: cycle-mid-execution-fail
  description: contract gate block in the build phase left the worktree predicate unsatisfied
  confidence: %.2f
  source: fixture
  type: failure-lesson
  category: episodic
  preventiveAction: re-dispatch the build with the contract escalation overlay
  failureContext:
    failedStep: build
    errorCategory: cycle-mid-execution-fail
    auditVerdict: FAIL
`, i, 0.9-float64(i)*0.05)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("recall-%02d.yaml", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func recallTestQuery() Query {
	return Query{
		Source:      "build",
		FailureMode: "contract gate block",
		Consequence: "cycle-mid-execution-fail",
		Keywords:    []string{"worktree", "predicate"},
	}
}

func lookupIDs(t *testing.T, kb KB) []string {
	t.Helper()
	got, err := kb.Lookup(context.Background(), recallTestQuery())
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	out := make([]string, 0, len(got))
	for _, l := range got {
		out = append(out, l.ID)
	}
	return out
}

// TestFileKB_ConfiguredRecallTakesRankingPrefix proves the bound is enforced by
// the KB AND that narrowing is a strict PREFIX of the existing deterministic
// ranking — the k highest-ranked lessons, not an arbitrary subset or a
// re-ranking under a different bound.
func TestFileKB_ConfiguredRecallTakesRankingPrefix(t *testing.T) {
	dir := recallCorpus(t, 7)

	full := lookupIDs(t, NewFileKB([]string{dir}))
	if len(full) != 5 {
		t.Fatalf("fixture broken: default recall returned %d lessons over a 7-match corpus, want 5", len(full))
	}

	bounded := lookupIDs(t, NewFileKBWithRecall([]string{dir}, 3))
	if len(bounded) != 3 {
		t.Fatalf("recall=3 returned %d lessons, want 3", len(bounded))
	}
	for i := range bounded {
		if bounded[i] != full[i] {
			t.Errorf("bounded[%d] = %q, want %q — the bound must be a top-k prefix of the same ranking", i, bounded[i], full[i])
		}
	}
}

// TestFileKB_DefaultConstructorRecallUnchanged is the anti-regression half: the
// existing constructor — the one every caller used before the knob existed —
// must still return 5.
func TestFileKB_DefaultConstructorRecallUnchanged(t *testing.T) {
	dir := recallCorpus(t, 7)
	if got := lookupIDs(t, NewFileKB([]string{dir})); len(got) != 5 {
		t.Errorf("NewFileKB Lookup returned %d lessons, want 5 (existing callers must see no behaviour change)", len(got))
	}
}

// TestFileKB_NonPositiveRecallFallsBackToDefault pins the degradation an
// operator typo must NOT be able to cause: a zero or negative recall silently
// disabling the advisor's recall memory.
func TestFileKB_NonPositiveRecallFallsBackToDefault(t *testing.T) {
	dir := recallCorpus(t, 7)
	for _, k := range []int{0, -1} {
		if got := lookupIDs(t, NewFileKBWithRecall([]string{dir}, k)); len(got) != 5 {
			t.Errorf("NewFileKBWithRecall(%d) returned %d lessons, want the default 5", k, len(got))
		}
	}
}

// TestFileKB_RecallAboveCorpusSizeReturnsAll checks the upper edge: a bound
// larger than the match count is not an error and does not pad the result.
func TestFileKB_RecallAboveCorpusSizeReturnsAll(t *testing.T) {
	dir := recallCorpus(t, 3)
	if got := lookupIDs(t, NewFileKBWithRecall([]string{dir}, 25)); len(got) != 3 {
		t.Errorf("recall=25 over a 3-lesson corpus returned %d lessons, want 3", len(got))
	}
}

// TestFileKB_ZeroValueRecallStillRecalls names the struct-literal path: a
// FileKB built without a constructor has recall 0, which must mean "built-in",
// never "none".
func TestFileKB_ZeroValueRecallStillRecalls(t *testing.T) {
	dir := recallCorpus(t, 7)
	if got := lookupIDs(t, &FileKB{roots: []string{dir}}); len(got) != 5 {
		t.Errorf("zero-value FileKB returned %d lessons, want the default 5", len(got))
	}
}
