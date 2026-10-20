package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

const (
	boundaryRatchetVerb   = "ratchet-tighten"
	boundaryRatchetDesc   = "tighten the size-ratchet allowances on origin/main"
	ratchetTightenMessage = "chore: tighten the size-ratchet allowances"
	ratchetGitTimeout     = 2 * time.Minute
)

func runBoundaryRatchetTighten(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var projectRoot string
	if _, err := (cliFlags{values: map[string]*string{"--project-root": &projectRoot}}).parse(args); err != nil || projectRoot == "" {
		fmt.Fprintf(stderr, "%s%s needs --project-root\n", boundaryPrefix, boundaryRatchetVerb)
		return exitUsage
	}
	if err := landRatchetTighten(projectRoot, stdout); err != nil {
		fmt.Fprintf(stderr, "%sWARN: ratchet tighten: %v\n", boundaryPrefix, err)
	}
	return 0
}

func landRatchetTighten(root string, stdout io.Writer) (err error) {
	if err := refuseBoundaryWrite(root, os.Getenv, time.Now(), "the offender list"); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "evolve-ratchet-tighten-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	wt := filepath.Join(dir, "main")
	if err := runGitSteps(root, []string{"fetch", "--quiet", "origin", "main"}, []string{"worktree", "add", "--quiet", "--detach", wt, "origin/main"}); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, runGitSteps(root, []string{"worktree", "remove", "--force", wt})) }()
	return tightenAndPush(wt, stdout)
}

func tightenAndPush(wt string, stdout io.Writer) error {
	module := filepath.Join(wt, "go")
	changes, encoded, err := sizeratchet.TightenModule(module)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Fprintln(stdout, "ratchet tighten: tight")
		return nil
	}
	err = os.WriteFile(filepath.Join(module, filepath.FromSlash(sizeratchet.OffendersRelPath)), encoded, 0o644)
	if err == nil {
		err = sizeratchet.Scan(module)
	}
	if err == nil {
		err = runGitSteps(wt,
			[]string{"commit", "--quiet", "-m", ratchetTightenMessage, "--", "go/" + sizeratchet.OffendersRelPath},
			[]string{"push", "--quiet", "origin", "HEAD:refs/heads/main"})
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s\nratchet tighten: landed on origin/main\n", strings.Join(changes, "\n"))
	return nil
}

func runGitSteps(dir string, steps ...[]string) error {
	git := gitexec.Default(dir)
	for _, args := range steps {
		ctx, cancel := context.WithTimeout(context.Background(), ratchetGitTimeout)
		err := git.Run(ctx, args...)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}
