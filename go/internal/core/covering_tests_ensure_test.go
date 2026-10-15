package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoveringTests_AbsentCorpusIsAnnouncedNotSilent(t *testing.T) {
	t.Parallel()

	worktree, workspace := t.TempDir(), t.TempDir()
	var sink injectedStderr
	ensureCoveringTests(context.Background(), worktree, workspace, &sink)
	out := sink.String()

	if _, err := os.Stat(filepath.Join(workspace, coveringTestsArtifact)); err == nil {
		t.Fatal("an underivable diff produced a corpus — fail-open must write nothing, not a fabricated one")
	}
	if !strings.Contains(out, "WARN covering-tests") {
		t.Errorf("no operator signal when the corpus is absent:\n\t%q\ntest-amplification then runs an UNSCOPED whole-repo search and nothing at the console distinguishes that run from a scoped one, which makes the token measurement the corpus exists to prove uninterpretable", out)
	}
	if !strings.Contains(out, "UNSCOPED") {
		t.Errorf("the warning does not say what actually degraded:\n\t%q", out)
	}
}

func TestCoveringTests_AvailableToTestAmplificationWithoutFreshBuild(t *testing.T) {
	worktree, workspace := t.TempDir(), t.TempDir()
	path := filepath.Join(workspace, coveringTestsArtifact)

	derived := 0
	orig := coveringTestsDeriver
	t.Cleanup(func() { coveringTestsDeriver = orig })
	coveringTestsDeriver = func(ctx context.Context, wt, ws string, _ io.Writer) {
		derived++
		if err := os.WriteFile(filepath.Join(ws, coveringTestsArtifact), []byte("# Covering Tests\n"), 0o644); err != nil {
			t.Fatalf("stub write: %v", err)
		}
	}

	var sink injectedStderr
	ensureCoveringTests(context.Background(), worktree, workspace, &sink)
	out := sink.String()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no corpus after ensure: %v — test-amplification is only scoped on paths where a build happened to complete in-process, which is exactly the hole", err)
	}
	if derived != 1 {
		t.Errorf("deriver ran %d time(s), want 1", derived)
	}
	if strings.Contains(out, "WARN") {
		t.Errorf("warned about a corpus it successfully derived:\n\t%q", out)
	}

	var again injectedStderr
	ensureCoveringTests(context.Background(), worktree, workspace, &again)
	out = again.String()
	if derived != 1 {
		t.Errorf("deriver ran again (%d) with the corpus already present — every post-phase normalize would re-pay the derivation", derived)
	}
	if strings.Contains(out, "WARN") {
		t.Errorf("warned with the corpus present:\n\t%q", out)
	}
}
