package qualitypanel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type GitRevertRunner struct {
	Repo   string
	Commit string
}

func (g GitRevertRunner) Revert(scratchDir string, h Hunk) (outcome RevertOutcome, err error) {
	if _, err := runIn("", "", "git", "-C", g.Repo, "worktree", "add", "--detach", scratchDir, g.Commit); err != nil {
		return "", err
	}
	defer func() {
		_, rmErr := runIn("", "", "git", "-C", g.Repo, "worktree", "remove", "--force", scratchDir)
		err = errors.Join(err, rmErr)
	}()
	if _, err := runIn(scratchDir, h.Patch, "git", "apply", "-R"); err != nil {
		return "", err
	}
	out, err := runIn(filepath.Join(scratchDir, h.Package), "", "go", lanebudget.Args([]string{"test", "-count=1", "."})...)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return RevertTestsPassed, nil
	case !errors.As(err, &exit):
		return "", err
	case strings.Contains(out, "[build failed]"):
		return RevertNotCompiled, nil
	}
	return RevertTestsFailed, nil
}

func runIn(dir, stdin, name string, args ...string) (string, error) {
	cmd := sysexec.Command(context.Background(), name, args...)
	cmd.Dir, cmd.Stdin = dir, strings.NewReader(stdin)
	if name == "go" {
		cmd.Env = lanebudget.Env(ipcenv.GoTestEnv())
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return buf.String(), nil
}
