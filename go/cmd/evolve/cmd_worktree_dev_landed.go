package main

import (
	"context"
	"io"
	"os"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/landed"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type devTreeState struct {
	head, base, diff string
	untracked        []landed.UntrackedFile
}

func (s devTreeState) equal(o devTreeState) bool {
	return s.head == o.head && s.base == o.base && s.diff == o.diff && slices.Equal(s.untracked, o.untracked)
}

func withGitEnv(run sysexec.RunFunc, extra ...string) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if env == nil {
			env = os.Environ()
		}
		return run(ctx, name, dir, args, append(slices.Clone(env), extra...), stdin, stdout, stderr)
	}
}

func devReadGit(dir string) gitexec.Git {
	return gitexec.Git{Dir: dir, Exec: withGitEnv(sysexec.DefaultRunner, "GIT_OPTIONAL_LOCKS=0")}
}

func readDevTreeState(ctx context.Context, tree gitexec.Git, head string) (devTreeState, error) {
	base, err := tree.Output(ctx, "merge-base", head, devOriginMain)
	if err != nil {
		return devTreeState{}, err
	}
	st := devTreeState{head: head, base: base}
	if st.diff, err = tree.Output(ctx, "diff", "--binary", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-color", "--no-relative", "--src-prefix=a/", "--dst-prefix=b/", st.base); err != nil {
		return devTreeState{}, err
	}
	st.untracked, err = landed.ListUntracked(ctx, tree)
	return st, err
}

func worktreeLandedInMain(ctx context.Context, tree gitexec.Git, st devTreeState) (landed.Verdict, error) {
	v, err := landed.Changes(ctx, tree, st.base)
	if err != nil || !v.Landed {
		return v, err
	}
	return landed.Untracked(ctx, tree, st.untracked)
}
