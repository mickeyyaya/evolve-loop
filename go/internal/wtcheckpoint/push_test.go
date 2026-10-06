package wtcheckpoint_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestPush_SendsOnlyTheNamedCheckpointRefsAndSaveNeverPushes(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	origin := gittest.Bare(t)
	h.store.Git("remote", "add", "origin", origin.Dir)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "a.txt", "work in progress\n")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})

	if got := origin.Git("for-each-ref", "--format=%(refname)"); got != "" {
		t.Fatalf("origin refs after a plain save = %q, want none: pushing WIP is never the default", got)
	}
	if err := wtcheckpoint.Push(context.Background(), h.hub(), []string{saved.Ref}); err != nil {
		t.Fatal(err)
	}
	if got := origin.Git("for-each-ref", "--format=%(refname) %(objectname)"); got != saved.Ref+" "+h.git(w.Dir, "rev-parse", saved.Ref) {
		t.Errorf("origin refs = %q, want exactly %s", got, saved.Ref)
	}
	err := wtcheckpoint.Push(context.Background(), h.hub(), []string{"refs/heads/feat/task"})
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("Push of a branch err = %v, want a refusal: only refs/checkpoints/ may be pushed", err)
	}
	if got := origin.Git("for-each-ref", "--format=%(refname)", "refs/heads"); got != "" {
		t.Errorf("origin branches = %q, want none", got)
	}
}
