package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestRepoContractFloorJudge_GivesANamedRedAsLinesAndAnUnnamedExitAsAnError(t *testing.T) {
	t.Parallel()
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t), Stderr: &injectedStderr{}}
	if lines, err := RepoContractFloorJudge((&fakeRepoContractPack{reds: []string{"pkg.TestX"}}).run)(context.Background(), in); err != nil || len(lines) != 1 || !strings.Contains(lines[0], "pkg.TestX") {
		t.Errorf("judge over a named red = %q, %v, want one line that names pkg.TestX and no error", lines, err)
	}
	unnamed := &fakeRepoContractPack{diagnostic: "go: toolchain chatter\n", err: errors.New("signal: killed")}
	if lines, err := RepoContractFloorJudge(unnamed.run)(context.Background(), in); err == nil || len(lines) != 0 {
		t.Errorf("judge over an exit that names no test = %q, %v, want no lines and an error for FailOpen to warn", lines, err)
	}
}

func TestReviewInput_MainRedSinkIsTheInjectedSink(t *testing.T) {
	t.Parallel()
	called := false
	var sink MainRedSink = func(string, baseattr.Verdict) (bool, error) {
		called = true
		return true, nil
	}
	opened, err := ReviewInput{MainRedStop: sink}.mainRedSink()(t.TempDir(), baseattr.Verdict{})
	if !called || !opened || err != nil {
		t.Errorf("the sink of the review = opened %v, err %v, called %v, want the injected sink", opened, err, called)
	}
}
