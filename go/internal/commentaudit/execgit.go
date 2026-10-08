package commentaudit

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type ExecGit struct{}

func (ExecGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := runGit("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := runGit("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (ExecGit) Show(base, path string) ([]byte, error) {
	return ReadAtBase(runGit, base)(path)
}

func (ExecGit) Root() (string, error) {
	out, err := runGit("rev-parse", "--show-toplevel")
	return strings.TrimSpace(string(out)), err
}

func runGit(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := sysexec.Command(context.Background(), "git", args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
