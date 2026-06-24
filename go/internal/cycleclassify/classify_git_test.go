package cycleclassify

import (
	"context"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// gitLogMatchesCycle is the testable core behind the gitLogFn seam. These
// white-box tests pin the exact git invocation and the match/no-match/error
// semantics that the raw exec.Command form could not unit-test (it shelled out
// to the real repo). Behavior parity with the original:
//   - any non-zero exit / error => false
//   - empty stdout (no matching commit) => false
//   - non-empty stdout => true
func TestGitLogMatchesCycle_Match_ReturnsTrue(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git log": {Stdout: "deadbeefcafe\n"}, // a commit matched
	}}
	g := gitexec.Git{Exec: fake.Run}

	if !gitLogMatchesCycle(context.Background(), g, "5") {
		t.Errorf("gitLogMatchesCycle = false, want true (commit present)")
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git log"}) {
		t.Fatalf("calls = %v, want [git log]", keys)
	}
	wantArgs := []string{"log", "--grep=cycle 5", "--format=%H", "main"}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("args = %v, want %v", got, wantArgs)
	}
}

func TestGitLogMatchesCycle_NoCommit_ReturnsFalse(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{} // zero value => empty stdout, exit 0
	if gitLogMatchesCycle(context.Background(), gitexec.Git{Exec: fake.Run}, "7") {
		t.Errorf("gitLogMatchesCycle = true, want false (no matching commit)")
	}
}

func TestGitLogMatchesCycle_GitError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git log": {ExitCode: 1}, // not a repo / git failure
	}}
	if gitLogMatchesCycle(context.Background(), gitexec.Git{Exec: fake.Run}, "9") {
		t.Errorf("gitLogMatchesCycle = true, want false (git exited non-zero)")
	}
}
