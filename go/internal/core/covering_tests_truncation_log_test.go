package core

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
