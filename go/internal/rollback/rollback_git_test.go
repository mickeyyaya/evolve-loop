package rollback

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/gitexec"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

func TestDeleteRemoteTagWith_NotPresent_DeletesLocalReturnsNotPresent(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git ls-remote": {Stdout: ""}, // tag absent on remote
	}}
	g := gitexec.Git{Dir: "/repo", Exec: fake.Run}

	if got := deleteRemoteTagWith(g, "v1.2.3"); got != "not-present" {
		t.Errorf("status = %q, want not-present", got)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git ls-remote", "git tag"}) {
		t.Errorf("calls = %v, want [git ls-remote, git tag] (best-effort local delete)", keys)
	}
}

func TestDeleteRemoteTagWith_Present_PushesAndDeletes(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git ls-remote": {Stdout: "abc123\trefs/tags/v1.2.3\n"}, // present on remote
	}}
	g := gitexec.Git{Dir: "/repo", Exec: fake.Run}

	if got := deleteRemoteTagWith(g, "v1.2.3"); got != "deleted" {
		t.Errorf("status = %q, want deleted", got)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git ls-remote", "git push", "git tag"}) {
		t.Fatalf("calls = %v, want [git ls-remote, git push, git tag]", keys)
	}
	if push := fake.Calls[1].Args; !reflect.DeepEqual(push, []string{"push", "origin", ":refs/tags/v1.2.3"}) {
		t.Errorf("push args = %v, want push origin :refs/tags/v1.2.3", push)
	}
}

func TestDeleteRemoteTagWith_PushFails_ReturnsFailed(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git ls-remote": {Stdout: "abc\trefs/tags/v1.2.3\n"},
		"git push":      {ExitCode: 1},
	}}
	if got := deleteRemoteTagWith(gitexec.Git{Dir: "/r", Exec: fake.Run}, "v1.2.3"); got != "failed" {
		t.Errorf("status = %q, want failed", got)
	}
	// A failed push must NOT fall through to a local `tag -d` (the remote tag
	// still exists — deleting the local one would mask the dangling tag).
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git ls-remote", "git push"}) {
		t.Errorf("calls = %v, want [git ls-remote, git push] (no tag -d after a failed push)", keys)
	}
}

func TestRevertAndShipWith_RevertFails_ReturnsFailed(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git revert": {ExitCode: 1},
	}}
	g := gitexec.Git{Dir: "/r", Exec: fake.Run}

	if got := revertAndShipWith(g, "/r", "deadbeef", "boom", "1.2.3"); got != "failed" {
		t.Errorf("status = %q, want failed", got)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git revert"}) {
		t.Errorf("calls = %v, want only [git revert] (must not reach ship on a failed revert)", keys)
	}
}

func TestRevertAndShipWith_RevertOK_NoBinary_LocalOnly(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")         // defeat resolveEvolveBin's LookPath fallback -> no binary
	fake := &fixtures.FakeExec{} // git revert succeeds (zero value)
	g := gitexec.Git{Dir: t.TempDir(), Exec: fake.Run}

	if got := revertAndShipWith(g, t.TempDir(), "deadbeef", "boom", "1.2.3"); got != "local-only" {
		t.Errorf("status = %q, want local-only (revert ok, no evolve binary)", got)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git revert"}) {
		t.Errorf("calls = %v, want [git revert]", keys)
	}
}
