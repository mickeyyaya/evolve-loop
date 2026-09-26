// Command commentaudit ranks packages by comment load and proves an edit
// touched only comments. See docs/conventions/code-comments.md.
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

func main() {
	os.Exit(commentaudit.Main(os.Args[1:], os.Stdout, os.Stderr, execGit{}))
}

type execGit struct{}

func (execGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := git("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (execGit) Show(base, path string) ([]byte, error) {
	if _, err := git("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return git("show", base+":"+path)
}

func git(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (execGit) Root() (string, error) {
	out, err := git("rev-parse", "--show-toplevel")
	return strings.TrimSpace(string(out)), err
}
