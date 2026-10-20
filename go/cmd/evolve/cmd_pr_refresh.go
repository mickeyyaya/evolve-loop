package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	prRefreshUsage   = "usage: evolve pr refresh [--project-root P]"
	prRefreshPrefix  = "evolve pr refresh: "
	prRefreshMessage = "chore: regenerate the derived files"
	gitVerifyMissing = 1
)

var (
	prRefreshEntries  = []string{"signal-codes", "darwin-set", "research-index"}
	prRefreshWorktree = core.DerivedWorktree
	prRefreshOpHeads  = []string{"MERGE_HEAD", "REBASE_HEAD", "CHERRY_PICK_HEAD"}
)

func runPRRefresh(args []string, stdout, stderr io.Writer) int {
	var projectRoot string
	operands, err := cliFlags{values: map[string]*string{"--project-root": &projectRoot}}.parse(args)
	if err != nil || len(operands) > 0 {
		fmt.Fprintln(stderr, prRefreshUsage)
		return exitUsage
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prRefreshPrefix, err)
		return exitIO
	}
	ctx := context.Background()
	g := gitexec.Default(root)
	outputs, err := refreshEntries(ctx, prRefreshWorktree(root), g)
	if err == nil {
		err = landRefreshed(ctx, g, outputs, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prRefreshPrefix, err)
		return exitIO
	}
	return 0
}

func refreshEntries(ctx context.Context, w derived.Worktree, g gitexec.Git) ([]string, error) {
	unmerged, err := gitOutput(ctx, g, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var outputs []string
	for _, name := range prRefreshEntries {
		e, ok := derived.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("the derived catalog has no entry %q", name)
		}
		refresh := w.Refresh
		if slices.ContainsFunc(e.Pathspecs(), func(p string) bool { return slices.Contains(strings.Fields(unmerged), p) }) {
			refresh = w.Regenerate
		}
		if err := refresh(ctx, e); err != nil {
			return nil, err
		}
		outputs = append(outputs, e.Pathspecs()...)
	}
	return outputs, nil
}

func landRefreshed(ctx context.Context, g gitexec.Git, outputs []string, stdout io.Writer) error {
	status, err := gitOutput(ctx, g, append([]string{"status", "--porcelain", "--"}, outputs...)...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		fmt.Fprintf(stdout, "%severy derived file is up to date\n", prRefreshPrefix)
		return nil
	}
	if _, err := gitOutput(ctx, g, append([]string{"add", "--"}, outputs...)...); err != nil {
		return err
	}
	op, err := operationInProgress(ctx, g)
	if err != nil {
		return err
	}
	if op != "" {
		fmt.Fprintf(stdout, "%sstaged %s; %s is in progress, so continue it\n", prRefreshPrefix, strings.Join(outputs, ", "), op)
		return nil
	}
	if _, err := gitOutput(ctx, g, append([]string{"commit", "-q", "-m", prRefreshMessage, "--"}, outputs...)...); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%scommitted %s\n", prRefreshPrefix, strings.Join(outputs, ", "))
	return nil
}

func operationInProgress(ctx context.Context, g gitexec.Git) (string, error) {
	for _, head := range prRefreshOpHeads {
		_, stderr, code, err := g.Capture(ctx, "rev-parse", "-q", "--verify", head)
		switch {
		case err == nil && code == 0:
			return head, nil
		case err != nil || code != gitVerifyMissing:
			return "", fmt.Errorf("git rev-parse --verify %s: rc=%d err=%v: %s", head, code, err, strings.TrimSpace(stderr))
		}
	}
	return "", nil
}

func gitOutput(ctx context.Context, g gitexec.Git, args ...string) (string, error) {
	stdout, stderr, code, err := g.Capture(ctx, args...)
	if err != nil || code != 0 {
		return "", fmt.Errorf("git %s: rc=%d err=%v: %s", args[0], code, err, strings.TrimSpace(stderr))
	}
	return stdout, nil
}
