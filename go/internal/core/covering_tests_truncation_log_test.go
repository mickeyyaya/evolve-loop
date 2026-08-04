package core

// RED contract for cycle-1267 Task 1 (`scope-test-amplification-context`) —
// the "no silent caps" half of the inbox item's how_to_apply:
//
//	cap corpus bytes with a loud truncation note (no silent caps)
//
// Today the cap is enforced and a `TRUNCATED:` note is written INTO the
// artifact, so the amplification agent can see it. The OPERATOR cannot: nothing
// reaches the cycle log, so a lane whose corpus was trimmed to 60% is
// indistinguishable at the console from one that was injected whole — and the
// item's own success metric is a before/after token measurement, which a silent
// cap makes uninterpretable ("did the corpus shrink, or did the cap eat it?").
// A cap that only the consumer of the artifact can see is a silent cap from the
// operator's seat.
//
// Two pins below:
//
//  1. renderCoveringTests must REPORT how many paths it dropped, rather than
//     burying the fact in its own output. That count is the single source for
//     both the in-artifact note and the operator warning — deriving it twice
//     (e.g. by re-scanning the rendered string for "TRUNCATED:") would put two
//     answers to one question in the tree.
//  2. writeCoveringTests must emit that count to stderr when it is non-zero,
//     and stay silent when nothing was dropped (a warning on every clean cycle
//     is noise that trains the operator to ignore the real one).
//
// Pin (1) changes an UNEXPORTED signature, so the two existing call sites in
// covering_tests_artifact_test.go must be updated to the two-value form. That
// is a mechanical call-site update, not a weakening: their assertions stay
// byte-identical.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

// TestRenderCoveringTests_ReportsOmittedCount — AC7. The renderer is the only
// place that knows the cap arithmetic, so it is the only place that can report
// the drop. Under the cap it must report zero; over the cap the reported count
// must equal the number of paths actually missing from the output, so the
// warning an operator reads is the truth and not an estimate.
func TestRenderCoveringTests_ReportsOmittedCount(t *testing.T) {
	small := []string{"go/internal/foo/foo_test.go", "go/internal/bar/bar_test.go"}
	out, omitted := renderCoveringTests(small)
	if omitted != 0 {
		t.Errorf("renderCoveringTests(2 paths) reported %d omitted, want 0 — nothing was dropped", omitted)
	}
	if strings.Contains(out, "TRUNCATED:") {
		t.Errorf("a two-path corpus carries a truncation note:\n%s", out)
	}

	var many []string
	for i := 0; i < 5000; i++ {
		many = append(many, "go/internal/pkg/a_very_long_package_name_"+strconv.Itoa(i)+"_test.go")
	}
	out, omitted = renderCoveringTests(many)
	if omitted <= 0 {
		t.Fatalf("renderCoveringTests(5000 paths) reported %d omitted, want > 0 — the corpus cannot fit in %d bytes",
			omitted, coveringTestsMaxBytes)
	}
	if !strings.Contains(out, "TRUNCATED:") {
		t.Fatalf("truncated corpus carries no visible note — a trimmed list is indistinguishable from a complete one")
	}

	// The reported count must be the REAL drop, not a placeholder: exactly
	// len(many)-omitted of the paths may appear in the rendered corpus.
	var present int
	for _, p := range many {
		if strings.Contains(out, "`"+p+"`") {
			present++
		}
	}
	if present != len(many)-omitted {
		t.Errorf("renderCoveringTests emitted %d of %d paths but reported %d omitted (want %d) — "+
			"the operator warning would quote a count the artifact contradicts",
			present, len(many), omitted, len(many)-present)
	}
}

// TestWriteCoveringTests_WarnsLoudlyOnTruncation — AC8, the operator-visible
// half, driven through the REAL write seam over a REAL git worktree: a repo
// whose only changed paths are more covering tests than the cap can hold. The
// artifact must still be written (a trimmed corpus beats no corpus), AND stderr
// must carry a warning naming the exact number of omitted paths.
func TestWriteCoveringTests_WarnsLoudlyOnTruncation(t *testing.T) {
	repo := t.TempDir()
	gitInitCoveringFixture(t, repo)

	dir := filepath.Join(repo, "go", "internal", "big")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture package: %v", err)
	}
	// ~400 paths of ~60 rendered bytes each comfortably exceeds the 16 KiB cap.
	const fixtureFiles = 400
	for i := 0; i < fixtureFiles; i++ {
		name := "covering_amplification_fixture_" + strconv.Itoa(i) + "_test.go"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package big\n"), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	files := changedpkgs.CoveringTests(repo, []string{"./internal/big"})
	if len(files) != fixtureFiles {
		t.Fatalf("fixture derivation found %d covering tests, want %d — the fixture, not the seam, is wrong",
			len(files), fixtureFiles)
	}
	_, wantOmitted := renderCoveringTests(files)
	if wantOmitted <= 0 {
		t.Fatalf("fixture corpus of %d paths did not exceed the %d-byte cap; this test cannot observe truncation",
			fixtureFiles, coveringTestsMaxBytes)
	}

	ws := t.TempDir()
	stderr := captureStderr(t, func() {
		writeCoveringTests(context.Background(), repo, ws)
	})

	if _, err := os.Stat(filepath.Join(ws, coveringTestsArtifact)); err != nil {
		t.Fatalf("no %s written for a truncated corpus (%v) — a trimmed corpus still beats a blind whole-repo search",
			coveringTestsArtifact, err)
	}
	if !strings.Contains(strings.ToLower(stderr), "truncat") {
		t.Fatalf("truncation is SILENT in the operator log; stderr was:\n%q\n"+
			"the item requires a loud cap: an operator reading the cycle log must be able to tell a trimmed "+
			"corpus from a complete one, otherwise the before/after token measurement is uninterpretable", stderr)
	}
	if !strings.Contains(stderr, strconv.Itoa(wantOmitted)) {
		t.Errorf("truncation warning does not name the omitted count %d; stderr was:\n%q", wantOmitted, stderr)
	}
}

// TestWriteCoveringTests_SilentWhenNothingTruncated — the negative half. A
// warning on every clean cycle is noise that trains the operator to ignore the
// real one, so an untruncated corpus must produce no truncation warning at all.
func TestWriteCoveringTests_SilentWhenNothingTruncated(t *testing.T) {
	repo := t.TempDir()
	gitInitCoveringFixture(t, repo)

	dir := filepath.Join(repo, "go", "internal", "small")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "small_test.go"), []byte("package small\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ws := t.TempDir()
	stderr := captureStderr(t, func() {
		writeCoveringTests(context.Background(), repo, ws)
	})

	if _, err := os.Stat(filepath.Join(ws, coveringTestsArtifact)); err != nil {
		t.Fatalf("no %s written for a one-path corpus: %v", coveringTestsArtifact, err)
	}
	if strings.Contains(strings.ToLower(stderr), "truncat") {
		t.Fatalf("a one-path corpus reported truncation; stderr was:\n%q", stderr)
	}
}

// gitInitCoveringFixture makes dir a git repository so changedWorktreePaths can
// enumerate it. Only `git ls-files --others` is needed (every fixture file is
// untracked), so no commit and no user identity are required — which keeps the
// fixture independent of the host's git config.
//
// cmd.Dir is set rather than shelling bare `git`: a bare invocation resolves the
// repo from the process cwd, which differs between the main tree, a cycle
// worktree, and each fleet lane.
func gitInitCoveringFixture(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable in this environment (%v): %s", err, out)
	}
}
