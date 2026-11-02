package qualitypanel

import (
	"errors"
	"strings"
	"testing"
)

func TestAmpRevertSensitivity_MixedOutcomesAreCountedPerClass(t *testing.T) {
	t.Parallel()
	hunks := []Hunk{
		hunkOf("a/x.go", "f1\n"), hunkOf("a/x.go", "f2\n"), hunkOf("a/x.go", "p1\n"),
		hunkOf("a/x.go", "n1\n"), hunkOf("a/x.go", "p2\n"),
	}
	runner := &fakeRevertRunner{outcomes: map[string]RevertOutcome{
		"f1\n": RevertTestsFailed, "f2\n": RevertTestsFailed, "p1\n": RevertTestsPassed,
		"n1\n": RevertNotCompiled, "p2\n": RevertTestsPassed,
	}}
	res, err := MeasureRevertSensitivity(revertInputFor(t, hunks...), runner)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sensitive != 3 || res.Insensitive != 2 || len(res.UncompilableHunks) != 1 || len(res.InsensitiveHunks) != 2 {
		t.Errorf("sensitive=%d insensitive=%d uncompilable=%d insensitiveHunks=%d, want 3 2 1 2",
			res.Sensitive, res.Insensitive, len(res.UncompilableHunks), len(res.InsensitiveHunks))
	}
	if res.TQ2.N != 5 || res.TQ2.Interval != Wilson(3, 5) {
		t.Errorf("TQ2 n=%d interval=%+v, want 5 %+v", res.TQ2.N, res.TQ2.Interval, Wilson(3, 5))
	}
}

func TestAmpRevertSensitivity_ExcludedHunksDoNotEnterTheDenominator(t *testing.T) {
	t.Parallel()
	hunks := []Hunk{hunkOf("go/internal/proctree/a.go", "k\n"), hunkOf("a/x.go", "p\n")}
	runner := &fakeRevertRunner{fallback: RevertTestsPassed}
	res, err := MeasureRevertSensitivity(revertInputFor(t, hunks...), runner)
	if err != nil {
		t.Fatal(err)
	}
	if res.TQ2.N != 1 || len(res.Excluded) != 1 || len(runner.files) != 1 || runner.files[0] != "a/x.go" {
		t.Errorf("n=%d excluded=%d files=%v, want 1 1 [a/x.go]", res.TQ2.N, len(res.Excluded), runner.files)
	}
}

func TestAmpRevertSensitivity_ARunnerErrorIsWrappedAndKeepsTheCause(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := MeasureRevertSensitivity(revertInputFor(t, manyHunks(3)...), &fakeRevertRunner{err: boom})
	if err == nil || !errors.Is(err, boom) || !strings.Contains(err.Error(), "revert runner") {
		t.Errorf("err = %v, want a wrapped %q naming revert runner", err, boom)
	}
}

func TestAmpParseHunks_AMissingFinalNewlineStillClosesThePatch(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/p/a.go b/p/a.go\n--- a/p/a.go\n+++ b/p/a.go\n@@ -1 +1 @@\n-a\n+b"
	got := ParseHunks(diff)
	if len(got) != 1 || !strings.HasSuffix(got[0].Patch, "\n") || got[0].Package != "p" {
		t.Errorf("hunks = %+v, want one hunk in package p ending in a newline", got)
	}
}

func TestAmpParseHunks_OnlyTestAndNonGoFilesYieldNoHunks(t *testing.T) {
	t.Parallel()
	diff := strings.Join([]string{
		"diff --git a/p/a_test.go b/p/a_test.go", "--- a/p/a_test.go", "+++ b/p/a_test.go", "@@ -1 +1 @@", "-a", "+b",
		"diff --git a/p/go.mod b/p/go.mod", "--- a/p/go.mod", "+++ b/p/go.mod", "@@ -1 +1 @@", "-a", "+b",
		"diff --git a/p/a.gox b/p/a.gox", "--- a/p/a.gox", "+++ b/p/a.gox", "@@ -1 +1 @@", "-a", "+b",
	}, "\n") + "\n"
	if got := ParseHunks(diff); len(got) != 0 {
		t.Errorf("hunks = %+v, want none", got)
	}
}
